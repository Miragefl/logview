# 搜索语法增强设计

日期：2026-07-29
作者：老王 + fenglei
状态：待 review

## 背景

logview 当前搜索语法（`internal/tui/searchquery.go`）支持：裸词全字段搜索、`field:value`、`AND`/`OR`（隐式 AND）、`after:/before:` 时间范围。刚修复了 `field: value`（冒号后空格）和小写 `and/or` 的解析 bug。用户在 review 语法后提出 6 项增强需求。

## 目标

| # | 需求 | 决策 |
|---|---|---|
| 1 | level 匹配方式 | **保持现状**（精确匹配，EqualFold 大小写不敏感） |
| 2 | time 字段 | **移除** `time:`（`after:/before:` 已覆盖时间；`time:` 字符串包含语义易混） |
| 3 | 括号分组 | **支持** `()`，可嵌套 |
| 4 | 引号 value | **支持** `field:"a b"`，引号内 `AND/OR/NOT/()` 当字面 |
| 5 | NOT 排除 | **支持** `NOT` 关键字，大小写不敏感 |
| 6 | 裸词默认字段 | **改 message only**（breaking：不再搜全字段） |

## 语法规范

### 词法

```
timeout              裸词 → 仅 message 字段包含匹配
field:value          字段搜索（冒号后空格可选）
field:"a b"          引号 value（含空格；内部操作符/括号当字面）
"hello world"        裸引号 → message 包含 "hello world"
AND  OR  NOT         操作符（大小写不敏感：AND/and/And 等）
( ... )              分组，可嵌套
```

### 字段

| 字段 | 匹配 | 备注 |
|---|---|---|
| `message:` | 包含 | |
| `source:` | 包含 | |
| `traceId:` | 包含 | |
| `thread:` | 包含 | |
| `logger:` | 包含 | |
| `level:` | 精确（EqualFold） | 不变 |
| `after:HH:MM` | 时间 >= | 不变 |
| `before:HH:MM` | 时间 < | 不变 |

**移除 `time:`**——从 `fieldPrefixes` 删除。之后 `time:xxx` 不再是字段搜索，会退化成裸 keyword（搜字面 "time:xxx"），用户应改用 `after/before`。

### 优先级

括号 > `NOT` > `AND` > `OR`，相邻词隐式 AND。

### BNF

```
orExpr  := andExpr (OR andExpr)*
andExpr := notExpr ((AND)? notExpr)*       # 隐式 AND
notExpr := NOT notExpr | term              # NOT 可连用（NOT NOT A = A）
term    := '(' orExpr ')' | field:value | keyword | '"' ... '"'
```

### 例子

```
JF350                                # message 含 JF350
message:JF350                        # 同上（显式）
level:ERROR NOT timeout              # ERROR 级且 message 不含 timeout
(ERROR OR WARN) AND message:timeout  # 分组
message:"hello world"                # message 含 "hello world"
NOT level:DEBUG                      # level 非 DEBUG
after:09:00 before:10:00 ERROR       # 时间范围（不变）
```

## 实现设计

改动集中在 `internal/tui/searchquery.go`。保持现有递归下降风格。

### 1. tokenizer（`tokenize()`）

当前用 `strings.Fields` 按空格分割，无法处理引号和括号。**改为索引扫描**：

- **引号扫描**：遇到 `"`，读到下一个 `"`，整体作为一个 token 的 value。支持 `field:"..."`（field 前缀 + 引号 value）和裸 `"..."`。
- **括号**：`(` 和 `)` 作为独立 token（`tokLParen` / `tokRParen`），即使紧贴其它字符（如 `(ERROR` 拆成 `(` + `ERROR`）。
- **NOT**：识别 `NOT`（大小写不敏感，复用 `strings.ToUpper`），新增 `tokNot`。
- **移除 time:**：`fieldPrefixes` 删掉 `"time:"`。
- 保留：`AND/OR` 大小写不敏感、`field:` 后空格吞 value（刚修的）、`after/before` 特殊处理。

新增 token kind：
```go
const (
    tokKeyword tokenKind = iota
    tokField
    tokAnd
    tokOr
    tokNot       // 新增
    tokLParen    // 新增
    tokRParen    // 新增
)
```

### 2. parser（递归下降）

在现有 `parseOrExpr` → `parseAndExpr` → `parseTerm` 链中插入 `parseNotExpr` 层：

```
parseOrExpr  (OR 最低)
parseAndExpr (隐式 AND)
parseNotExpr (NOT)        ← 新增层
parseTerm    (括号 / field / keyword)
```

- **`parseTerm`**：遇到 `tokLParen` → 消费 `(`，递归 `parseOrExpr`，期望 `)`；否则原逻辑（field/keyword）。
- **`parseNotExpr`**：遇到 `tokNot` → 消费，递归 `parseNotExpr`（支持 `NOT NOT A`），返回 `notNode{child}`；否则 `parseTerm`。

### 3. AST

新增 `notNode`：
```go
type notNode struct{ child queryNode }

func (n *notNode) match(line *model.ParsedLine) bool { return !n.child.match(line) }
func (n *notNode) keywords() []string                { return nil } // NOT 不参与高亮
```

### 4. 裸词匹配改 message only

`termNode.match` 的 `keywordTerm` 分支，从搜 6 字段改为只搜 `message`：
```go
case keywordTerm:
    return containsIgnoreCase(line.Message, n.value)
```

`HighlightKeywords` 不变（裸词仍作为高亮关键词）。

## 测试（`internal/tui/searchquery_test.go`）

新增/更新：

| 测试 | 覆盖 |
|---|---|
| `TestParenGrouping` | `(ERROR OR WARN) AND timeout` |
| `TestNestedParens` | `(A AND (B OR C))` |
| `TestNotOperator` | `ERROR NOT timeout`、`NOT level:DEBUG` |
| `TestNotNot` | `NOT NOT ERROR` = `ERROR` |
| `TestQuotedValue` | `message:"hello world"` |
| `TestQuotedLiteral` | `"ERROR AND timeout"` 整体字面 |
| `TestBareWordMessageOnly` | 裸 `ERROR` 不命中 level=ERROR（breaking 验证） |
| 更新 `TestSimpleKeyword` | 改为 `level:ERROR` 或 message 含 ERROR |

保留：`TestAndOperator`/`TestOrOperator`/`TestImplicitAnd`/`TestAndOrPrecedence`/`TestFieldSpaceAndCaseInsensitiveOps`/`TestTimeRangeHint`（after/before 不变）。

## Breaking Changes

1. **裸词不再搜全字段**：`ERROR` 裸搜不再命中 `level=ERROR`（需 `level:ERROR`）；不搜 traceId/thread/logger/raw。用户主要受影响的是裸 `ERROR/WARN` 搜级别——改用 `level:ERROR`。
2. **`time:` 移除**：`time:09:30` 退化为裸 keyword。改用 `after/before`。

## 不在范围内（YAGNI）

- 引号内转义 `\"`（暂不支持，引号内不能再出现引号）
- 正则 / 通配符搜索
- 相对时间（`last 5min`）
- 日期（跨天）
- 字段别名（`msg:` 等）

## 文档

README "搜索语法"章节需更新：补括号/引号/NOT/裸词 message 语义，移除 time:。
