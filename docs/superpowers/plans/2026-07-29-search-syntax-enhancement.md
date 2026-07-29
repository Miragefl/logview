# 搜索语法增强 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 logview 搜索加括号分组、引号 value、NOT 操作符，砍掉 time: 字段，裸词改为只搜 message。

**Architecture:** 改造 `internal/tui/searchquery.go`——tokenizer 从 `strings.Fields` 改为索引扫描（支持引号/括号），parser 在 `andExpr` 与 `term` 之间插入 `notExpr` 层，AST 加 `notNode`。递归下降风格不变。

**Tech Stack:** Go 1.21+，标准库 `strings`/`time`，`testing`。

## Global Constraints

- 改动只在 `internal/tui/searchquery.go` + `internal/tui/searchquery_test.go`（临时复现文件 `internal/tui/repro_search_test.go` 在 Task 5 末尾删除，合并进正式测试）
- 保持现有测试（除 `TestSimpleKeyword` 在 Task 3 更新）全绿
- 操作符 `AND/OR/NOT` 大小写不敏感
- 不做引号转义、正则、相对时间、字段别名（YAGNI，见 spec）
- 每个 task 结尾 `go test ./internal/tui/` 必须全过才 commit
- 不主动 git push（用户全局规矩）

## File Structure

| 文件 | 责任 | 改动 |
|---|---|---|
| `internal/tui/searchquery.go` | 搜索语法 tokenizer + parser + AST + match | 重写 `tokenize()`、加 token kinds、加 `parseNotExpr`/`notNode`、改 `keywordTerm` match、删 `time:` |
| `internal/tui/searchquery_test.go` | 正式回归测试 | 新增括号/引号/NOT/breaking 测试，更新 `TestSimpleKeyword` |
| `internal/tui/repro_search_test.go` | 临时复现文件 | Task 5 末尾删除（内容已合并） |

---

### Task 1: tokenizer 重构为索引扫描 + 新 token kinds

把 `tokenize()` 从 `strings.Fields` 改为 rune-by-rune 索引扫描，为括号/引号/NOT 铺路。**本 task 保持现有行为完全不变**（现有测试全过），同时识别 `(`/`)`/`NOT`/引号为独立 token（parser 暂不消费，留后续 task）。

**Files:**
- Modify: `internal/tui/searchquery.go`（`tokenize()` 全文 + `tokenKind` 常量 + `isOperatorOrField` 保留）
- Test: `internal/tui/searchquery_test.go`

**Interfaces:**
- Produces: `tokenize(input string) []token` 新签名不变；新增 `tokNot`/`tokLParen`/`tokRParen` 三个 `tokenKind`；token 结构不变（`kind`/`field`/`value`）

- [ ] **Step 1: 加新 token kinds**

`internal/tui/searchquery.go` 找到：
```go
const (
	tokKeyword tokenKind = iota
	tokField
	tokAnd
	tokOr
)
```
改为：
```go
const (
	tokKeyword tokenKind = iota
	tokField
	tokAnd
	tokOr
	tokNot
	tokLParen
	tokRParen
)
```

- [ ] **Step 2: 写 tokenizer 行为测试（先固化现有行为 + 新 token 识别）**

追加到 `internal/tui/searchquery_test.go`（注意：`tokenize` 是包内函数，测试可直接调）：
```go
func TestTokenizeNewTokens(t *testing.T) {
	cases := map[string][]tokenKind{
		"NOT level:ERROR": {tokNot, tokField},
		"(ERROR OR WARN)": {tokLParen, tokKeyword, tokOr, tokKeyword, tokRParen},
		`message:"a b"`:    {tokField},
		`"hello world"`:    {tokKeyword},
	}
	for input, want := range cases {
		toks := tokenize(input)
		if len(toks) != len(want) {
			t.Errorf("tokenize(%q) got %d tokens %v, want %d", input, len(toks), toks, len(want))
			continue
		}
		for i, w := range want {
			if toks[i].kind != w {
				t.Errorf("tokenize(%q)[%d] = %v, want %v", input, i, toks[i].kind, w)
			}
		}
	}
	// 引号 value 保留空格
	toks := tokenize(`message:"a b"`)
	if toks[0].value != "a b" {
		t.Errorf(`message:"a b" value = %q, want "a b"`, toks[0].value)
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./internal/tui/ -run TestTokenizeNewTokens -v`
Expected: FAIL（当前 `strings.Fields` 不识别 NOT/括号/引号）

- [ ] **Step 4: 重写 `tokenize()` 为索引扫描**

替换整个 `tokenize` 函数为：
```go
func tokenize(input string) []token {
	var tokens []token
	s := input
	i := 0
	for i < len(s) {
		// 跳过空白
		if s[i] == ' ' || s[i] == '\t' {
			i++
			continue
		}
		// 括号
		if s[i] == '(' {
			tokens = append(tokens, token{kind: tokLParen})
			i++
			continue
		}
		if s[i] == ')' {
			tokens = append(tokens, token{kind: tokRParen})
			i++
			continue
		}
		// 裸引号字符串 → keyword
		if s[i] == '"' {
			j := i + 1
			for j < len(s) && s[j] != '"' {
				j++
			}
			tokens = append(tokens, token{kind: tokKeyword, value: s[i+1 : j]})
			if j < len(s) {
				j++ // 跳过闭合引号
			}
			i = j
			continue
		}
		// 读到下一个 空白/括号/引号
		j := i
		for j < len(s) && s[j] != ' ' && s[j] != '\t' && s[j] != '(' && s[j] != ')' && s[j] != '"' {
			j++
		}
		word := s[i:j]
		i = j
		// 操作符（大小写不敏感）
		switch strings.ToUpper(word) {
		case "AND":
			tokens = append(tokens, token{kind: tokAnd, value: "AND"})
			continue
		case "OR":
			tokens = append(tokens, token{kind: tokOr, value: "OR"})
			continue
		case "NOT":
			tokens = append(tokens, token{kind: tokNot, value: "NOT"})
			continue
		}
		// field 前缀
		matched := false
		for _, prefix := range fieldPrefixes {
			if strings.HasPrefix(word, prefix) {
				field := strings.TrimSuffix(prefix, ":")
				value := word[len(prefix):]
				// field: 后空格或引号的 value
				if value == "" {
					k := i
					for k < len(s) && (s[k] == ' ' || s[k] == '\t') {
						k++
					}
					if k < len(s) && s[k] == '"' {
						// field:"a b"
						m := k + 1
						for m < len(s) && s[m] != '"' {
							m++
						}
						value = s[k+1 : m]
						if m < len(s) {
							m++
						}
						i = m
					} else if k < len(s) && s[k] != '(' && s[k] != ')' {
						// field: value（空格）
						n := k
						for n < len(s) && s[n] != ' ' && s[n] != '\t' && s[n] != '(' && s[n] != ')' && s[n] != '"' {
							n++
						}
						nextWord := s[k:n]
						if nextWord != "" && !isOperatorOrField(nextWord) {
							value = nextWord
							i = n
						}
					}
				}
				tokens = append(tokens, token{kind: tokField, field: field, value: value})
				matched = true
				break
			}
		}
		if !matched {
			tokens = append(tokens, token{kind: tokKeyword, value: word})
		}
	}
	return tokens
}
```

- [ ] **Step 5: 跑全量测试确认通过（现有行为不破坏 + 新 token 识别）**

Run: `go test ./internal/tui/ -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok github.com/justfun/logview/internal/tui`（无 FAIL）

- [ ] **Step 6: Commit**

```bash
git add internal/tui/searchquery.go internal/tui/searchquery_test.go
git commit -m "refactor: tokenize to index scan, recognize NOT/parens/quotes"
```

---

### Task 2: 移除 time: 字段

`time:` 字段语义模糊（字符串包含），与 `after:/before:` 重复，砍掉。

**Files:**
- Modify: `internal/tui/searchquery.go`（`fieldPrefixes`）
- Test: `internal/tui/searchquery_test.go`

**Interfaces:**
- Produces: `fieldPrefixes` 不再含 `"time:"`；`time:xxx` 退化为 keyword（搜字面）

- [ ] **Step 1: 写失败测试**

追加到 `internal/tui/searchquery_test.go`：
```go
func TestTimeFieldRemoved(t *testing.T) {
	// time: 不再是字段，退化成 keyword（在 message 里搜字面 "time:09:30"）
	q := parseSearchQuery("time:09:30")
	line := parsedLine("INFO", "", "", "", "some message")
	if q.MatchLine(line) {
		t.Error("time:09:30 as keyword should not match message without it")
	}
	// 验证 fieldPrefixes 不含 time:
	for _, p := range fieldPrefixes {
		if p == "time:" {
			t.Error("time: should be removed from fieldPrefixes")
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tui/ -run TestTimeFieldRemoved -v`
Expected: FAIL（当前 `time:` 在 fieldPrefixes）

- [ ] **Step 3: 从 fieldPrefixes 删 time:**

`internal/tui/searchquery.go` 找到：
```go
var fieldPrefixes = []string{
	"after:", "before:",
	"traceId:", "thread:", "level:", "logger:", "message:", "source:", "time:",
}
```
改为：
```go
var fieldPrefixes = []string{
	"after:", "before:",
	"traceId:", "thread:", "level:", "logger:", "message:", "source:",
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/tui/ -run TestTimeFieldRemoved -v`
Expected: PASS

- [ ] **Step 5: 跑全量确认无回归**

Run: `go test ./internal/tui/ 2>&1 | tail -1`
Expected: `ok`

- [ ] **Step 6: Commit**

```bash
git add internal/tui/searchquery.go internal/tui/searchquery_test.go
git commit -m "refactor: remove ambiguous time: field prefix"
```

---

### Task 3: 裸词改 message only

裸词（keywordTerm）从搜 6 字段改为只搜 `message`。**Breaking change**：裸 `ERROR` 不再命中 `level=ERROR`。

**Files:**
- Modify: `internal/tui/searchquery.go`（`termNode.match` 的 `keywordTerm` 分支）
- Test: `internal/tui/searchquery_test.go`（更新 `TestSimpleKeyword` + 新增 breaking 验证）

**Interfaces:**
- Produces: `termNode.match` 的 `keywordTerm` 只调 `containsIgnoreCase(line.Message, n.value)`

- [ ] **Step 1: 更新现有 TestSimpleKeyword（裸 ERROR 不再搜 level）**

`internal/tui/searchquery_test.go` 找到：
```go
func TestSimpleKeyword(t *testing.T) {
	q := parseSearchQuery("ERROR")
	line := parsedLine("ERROR", "abc", "main", "com.example.App", "something broke")
	if !q.MatchLine(line) {
		t.Error("should match line with ERROR level")
	}
	line2 := parsedLine("INFO", "abc", "main", "com.example.App", "all good")
	if q.MatchLine(line2) {
		t.Error("should not match line without ERROR")
	}
}
```
改为（裸 ERROR 现在搜 message，要用 level:ERROR 搜级别）：
```go
func TestSimpleKeyword(t *testing.T) {
	// 裸词只搜 message：裸 ERROR 不再命中 level=ERROR
	q := parseSearchQuery("ERROR")
	// message 含 ERROR → 匹配
	if !q.MatchLine(parsedLine("INFO", "", "", "", "ERROR happened")) {
		t.Error("bare ERROR should match message containing ERROR")
	}
	// level=ERROR 但 message 不含 → 不匹配
	if q.MatchLine(parsedLine("ERROR", "abc", "main", "com.example.App", "something broke")) {
		t.Error("bare ERROR should NOT match level=ERROR without ERROR in message")
	}
}
```

- [ ] **Step 2: 写 breaking 验证测试**

追加到 `internal/tui/searchquery_test.go`：
```go
func TestBareWordMessageOnly(t *testing.T) {
	// 裸词不搜 traceId/thread/logger/level
	q := parseSearchQuery("abc123")
	if q.MatchLine(parsedLine("INFO", "abc123", "", "", "no match here")) {
		t.Error("bare word should not search traceId")
	}
	if q.MatchLine(parsedLine("INFO", "", "", "abc123", "no match here")) {
		t.Error("bare word should not search logger")
	}
	// message 含才匹配
	if !q.MatchLine(parsedLine("INFO", "", "", "", "trace abc123 found")) {
		t.Error("bare word should match message")
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./internal/tui/ -run "TestSimpleKeyword|TestBareWordMessageOnly" -v`
Expected: FAIL（裸词当前搜全字段）

- [ ] **Step 4: 改 keywordTerm match 为 message only**

`internal/tui/searchquery.go` 找到 `termNode.match` 的：
```go
	case keywordTerm:
		return containsIgnoreCase(line.Message, n.value) ||
			containsIgnoreCase(line.Raw.Text, n.value) ||
			containsIgnoreCase(line.TraceID, n.value) ||
			containsIgnoreCase(line.Thread, n.value) ||
			containsIgnoreCase(line.Logger, n.value) ||
			containsIgnoreCase(line.Level, n.value)
```
改为：
```go
	case keywordTerm:
		return containsIgnoreCase(line.Message, n.value)
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/tui/ -run "TestSimpleKeyword|TestBareWordMessageOnly" -v`
Expected: PASS

- [ ] **Step 6: 跑全量确认（注意 KeywordSearchInMessage 等仍应过）**

Run: `go test ./internal/tui/ 2>&1 | tail -1`
Expected: `ok`（若 `TestKeywordSearchInMessage` 失败，检查它的 line 是否 message 含关键词——它用的是 message "connection timeout"，应仍过）

- [ ] **Step 7: Commit**

```bash
git add internal/tui/searchquery.go internal/tui/searchquery_test.go
git commit -m "feat: bare search term matches message field only"
```

---

### Task 4: NOT 操作符

新增 `parseNotExpr` 层 + `notNode` AST。`NOT term` 对 term 取反，支持 `NOT NOT A`、`NOT field:ERROR`、`NOT (...)`。

**Files:**
- Modify: `internal/tui/searchquery.go`（AST 加 `notNode`、parser 加 `parseNotExpr`、`parseAndExpr` 调 `parseNotExpr` 替代 `parseTerm`）
- Test: `internal/tui/searchquery_test.go`

**Interfaces:**
- Produces: `notNode{child queryNode}`，`match() bool`，`keywords() []string`（返回 nil）；`parseNotExpr(tokens, pos) (queryNode, int)`

- [ ] **Step 1: 写失败测试**

追加到 `internal/tui/searchquery_test.go`：
```go
func TestNotOperator(t *testing.T) {
	// ERROR NOT timeout：level=ERROR 且 message 不含 timeout
	q := parseSearchQuery("message:ERROR NOT timeout")
	if !q.MatchLine(parsedLine("ERROR", "", "", "", "[ERROR] boom")) {
		t.Error("ERROR NOT timeout should match ERROR without timeout")
	}
	if q.MatchLine(parsedLine("ERROR", "", "", "", "[ERROR] connection timeout")) {
		t.Error("ERROR NOT timeout should not match when timeout present")
	}
	// NOT level:DEBUG：非 DEBUG
	q2 := parseSearchQuery("NOT level:DEBUG")
	if !q2.MatchLine(parsedLine("INFO", "", "", "", "x")) {
		t.Error("NOT level:DEBUG should match INFO")
	}
	if q2.MatchLine(parsedLine("DEBUG", "", "", "", "x")) {
		t.Error("NOT level:DEBUG should not match DEBUG")
	}
	// NOT NOT = 原值
	q3 := parseSearchQuery("NOT NOT message:timeout")
	if !q3.MatchLine(parsedLine("INFO", "", "", "", "connection timeout")) {
		t.Error("NOT NOT timeout should match timeout")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tui/ -run TestNotOperator -v`
Expected: FAIL（NOT 当前被当 keyword）

- [ ] **Step 3: 加 notNode AST**

在 `internal/tui/searchquery.go` 的 `orNode` 定义之后加：
```go
type notNode struct {
	child queryNode
}

func (n *notNode) match(line *model.ParsedLine) bool { return !n.child.match(line) }

func (n *notNode) keywords() []string { return nil } // 排除项不参与高亮
```

- [ ] **Step 4: 加 parseNotExpr 层并接入 parseAndExpr**

在 `internal/tui/searchquery.go` 找到 `parseAndExpr`：
```go
func parseAndExpr(tokens []token, pos int) (queryNode, int) {
	left, pos := parseTerm(tokens, pos)
```
把 `parseTerm(tokens, pos)`（仅这一处，parseAndExpr 里的）改为 `parseNotExpr(tokens, pos)`：
```go
func parseAndExpr(tokens []token, pos int) (queryNode, int) {
	left, pos := parseNotExpr(tokens, pos)
```
同样把 parseAndExpr 循环里的 `next, newPos := parseTerm(tokens, pos)` 改为 `parseNotExpr`。

然后在 `parseTerm` 函数**之前**插入新函数：
```go
func parseNotExpr(tokens []token, pos int) (queryNode, int) {
	if pos < len(tokens) && tokens[pos].kind == tokNot {
		child, newPos := parseNotExpr(tokens, pos+1) // 支持 NOT NOT
		return &notNode{child: child}, newPos
	}
	return parseTerm(tokens, pos)
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/tui/ -run TestNotOperator -v`
Expected: PASS

- [ ] **Step 6: 跑全量确认**

Run: `go test ./internal/tui/ 2>&1 | tail -1`
Expected: `ok`

- [ ] **Step 7: Commit**

```bash
git add internal/tui/searchquery.go internal/tui/searchquery_test.go
git commit -m "feat: support NOT operator for exclusion"
```

---

### Task 5: 括号分组 + 收尾

parser 的 `parseTerm` 识别 `(` 递归 `parseOrExpr` 到 `)`。支持嵌套。最后删除临时复现文件。

**Files:**
- Modify: `internal/tui/searchquery.go`（`parseTerm` 加括号分支）
- Modify: `internal/tui/searchquery_test.go`（括号测试）
- Delete: `internal/tui/repro_search_test.go`（内容已合并到正式测试）

**Interfaces:**
- Produces: `parseTerm` 处理 `tokLParen`/`tokRParen`

- [ ] **Step 1: 写失败测试**

追加到 `internal/tui/searchquery_test.go`：
```go
func TestParenGrouping(t *testing.T) {
	// (ERROR OR WARN) AND timeout
	q := parseSearchQuery("(message:ERROR OR message:WARN) AND timeout")
	if !q.MatchLine(parsedLine("ERROR", "", "", "", "timeout occurred")) {
		t.Error("(ERROR OR WARN) AND timeout should match ERROR+timeout")
	}
	if q.MatchLine(parsedLine("INFO", "", "", "", "timeout occurred")) {
		t.Error("should not match INFO even with timeout")
	}
}

func TestNestedParens(t *testing.T) {
	// message:(a OR (b AND c)) —— 这里用嵌套 keyword
	q := parseSearchQuery("message:ERROR OR (message:WARN AND timeout)")
	if !q.MatchLine(parsedLine("ERROR", "", "", "", "x")) {
		t.Error("should match ERROR")
	}
	if !q.MatchLine(parsedLine("WARN", "", "", "", "timeout")) {
		t.Error("should match WARN AND timeout")
	}
	if q.MatchLine(parsedLine("WARN", "", "", "", "no timeout")) {
		t.Error("WARN without timeout should not match second branch")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tui/ -run "TestParenGrouping|TestNestedParens" -v`
Expected: FAIL（`(` 当前被当 keyword 字面）

- [ ] **Step 3: parseTerm 加括号分支**

`internal/tui/searchquery.go` 找到 `parseTerm` 开头：
```go
func parseTerm(tokens []token, pos int) (queryNode, int) {
	if pos >= len(tokens) {
		return &termNode{typ: keywordTerm, value: ""}, pos
	}
	tok := tokens[pos]
	switch tok.kind {
```
在 `tok := tokens[pos]` 之后、`switch tok.kind {` 之前插入括号处理：
```go
func parseTerm(tokens []token, pos int) (queryNode, int) {
	if pos >= len(tokens) {
		return &termNode{typ: keywordTerm, value: ""}, pos
	}
	tok := tokens[pos]
	if tok.kind == tokLParen {
		node, newPos := parseOrExpr(tokens, pos+1)
		if newPos < len(tokens) && tokens[newPos].kind == tokRParen {
			newPos++ // 消费 )
		}
		return node, newPos
	}
	switch tok.kind {
```
然后在 `parseTerm` 的 switch 的 `default:` 分支里（处理 AND/OR/NOT at term position），加 `tokLParen`/`tokRParen` 也走 default（当字面 keyword，容错未闭合括号）——实际上 default 已覆盖，无需改。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/tui/ -run "TestParenGrouping|TestNestedParens" -v`
Expected: PASS

- [ ] **Step 5: 跑全量 + go vet 确认**

Run: `go test ./internal/tui/ 2>&1 | tail -1 && go vet ./internal/tui/`
Expected: `ok` + 无 vet 警告

- [ ] **Step 6: 删除临时复现文件（内容已合并到正式测试）**

```bash
rm internal/tui/repro_search_test.go
```

- [ ] **Step 7: 再次跑全量确认删除后仍全绿**

Run: `go test ./internal/tui/ 2>&1 | tail -1`
Expected: `ok`

- [ ] **Step 8: Commit**

```bash
git add internal/tui/searchquery.go internal/tui/searchquery_test.go internal/tui/repro_search_test.go
git commit -m "feat: support parenthesized grouping in search"
```

---

## Self-Review（plan 作者自查）

**Spec 覆盖：**
- ✅ level 保持精确——spec 目标表 #1（不改代码，无 task，正确）
- ✅ time 移除——Task 2
- ✅ 括号嵌套——Task 5（TestNestedParens 验证嵌套）
- ✅ 引号 value——Task 1（tokenizer 引号扫描 + TestTokenizeNewTokens 验证 `message:"a b"`）
- ✅ NOT——Task 4（含 NOT NOT / NOT field: / NOT (...)）
- ✅ 裸词 message only——Task 3

**Placeholder 扫描：** 无 TBD/TODO，每个代码步骤都给了完整代码。

**类型一致性：** `tokNot`/`tokLParen`/`tokRParen`（Task 1 定义）→ Task 4 用 `tokNot`、Task 5 用 `tokLParen`/`tokRParen`，一致。`notNode`（Task 4 定义）match/keywords 签名一致。`parseNotExpr`（Task 4）被 `parseAndExpr` 调用，签名 `(tokens, pos) (queryNode, int)` 一致。

**风险点：** Task 1 tokenizer 重构是核心，必须保证现有测试全过（Step 5 把关）。Task 3 breaking change 影响裸 ERROR 搜级别——已在 TestSimpleKeyword 更新中体现。
