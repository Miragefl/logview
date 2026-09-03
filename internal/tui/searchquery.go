package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/justfun/logview/internal/model"
)

// --- AST node types ---

type queryNode interface {
	match(line *model.ParsedLine) bool
	keywords() []string
}

type termType int

const (
	keywordTerm termType = iota
	fieldTerm
	timeTerm  // time: 条件（op + 绝对时刻；time2 非空为闭区间）
	errorTerm // time: 值解析错误（匹配恒 false，hint 红字提示）
)

type termNode struct {
	typ    termType
	field  string
	value  string
	time   *time.Time
	time2  *time.Time // 区间上界（闭区间 time:a..b）
	op     string     // ">" "<" ">=" "<="
	errMsg string     // errorTerm 的错误说明
}

func (n *termNode) match(line *model.ParsedLine) bool {
	switch n.typ {
	case keywordTerm:
		return containsIgnoreCase(line.Message, n.value)
	case fieldTerm:
		val := line.Get(model.Field(n.field))
		if val == "" {
			return false
		}
		if n.field == "level" {
			return strings.EqualFold(val, n.value)
		}
		return containsIgnoreCase(val, n.value)
	case timeTerm:
		if line.Time.IsZero() || n.time == nil {
			return false // 无时间字段的行不参与任何 time 条件（NULL 语义）
		}
		if line.Time.Year() <= 0 {
			return n.matchHMS(line.Time) // 无日期时间戳（如 HH:mm:ss.SSS 日志）：按当日时分窗口退化
		}
		lt := line.Time
		if n.time2 != nil { // 闭区间
			return !lt.Before(*n.time) && !lt.After(*n.time2)
		}
		switch n.op {
		case ">":
			return lt.After(*n.time)
		case ">=":
			return !lt.Before(*n.time)
		case "<":
			return lt.Before(*n.time)
		case "<=":
			return !lt.After(*n.time)
		}
		return true
	case errorTerm:
		return false
	}
	return true
}

// matchHMS 无日期时间戳行的退化匹配：行与查询值都取当日时分秒比较。
// 日志时间戳无时区、按用户本地视角解析（parser 侧 ParseInLocation(time.Local)），
// 查询锚点同为本地——同 location 下字面即时分语义，无跨时区偏移。
// 跨午夜窗口有歧义（23:50 vs >00:10），属无日期日志的固有限制。
func (n *termNode) matchHMS(lt time.Time) bool {
	ls := hmsOfDay(lt)
	if n.time2 != nil { // 区间糖
		lo, hi := hmsOfDay(*n.time), hmsOfDay(*n.time2)
		if lo > hi {
			lo, hi = hi, lo
		}
		return ls >= lo && ls <= hi
	}
	ts := hmsOfDay(*n.time)
	switch n.op {
	case ">":
		return ls > ts
	case ">=":
		return ls >= ts
	case "<":
		return ls < ts
	case "<=":
		return ls <= ts
	}
	return true
}

// hmsOfDay 折叠为当日秒数（0..86399）。
func hmsOfDay(t time.Time) int {
	return t.Hour()*3600 + t.Minute()*60 + t.Second()
}

func (n *termNode) keywords() []string {
	switch n.typ {
	case keywordTerm:
		return []string{n.value}
	case fieldTerm:
		return []string{n.value}
	default:
		return nil
	}
}

type andNode struct {
	children []queryNode
}

func (n *andNode) match(line *model.ParsedLine) bool {
	for _, c := range n.children {
		if !c.match(line) {
			return false
		}
	}
	return true
}

func (n *andNode) keywords() []string {
	var kw []string
	for _, c := range n.children {
		kw = append(kw, c.keywords()...)
	}
	return kw
}

type orNode struct {
	children []queryNode
}

func (n *orNode) match(line *model.ParsedLine) bool {
	for _, c := range n.children {
		if c.match(line) {
			return true
		}
	}
	return false
}

func (n *orNode) keywords() []string {
	var kw []string
	for _, c := range n.children {
		kw = append(kw, c.keywords()...)
	}
	return kw
}

type notNode struct {
	child queryNode
}

func (n *notNode) match(line *model.ParsedLine) bool {
	// NULL 传播：无时间字段行不参与任何 time 条件（含 NOT 包裹，取反不得复活）。
	// 深层子树同样传播：NOT NOT time:x / NOT (time:x OR ...) 不得把无时间行翻回来。
	// 取舍：不做严格三值逻辑（NOT (time:x AND f) 对无时间且 f=false 的行按"未知→排除"处理）。
	if line.Time.IsZero() && containsTimeTerm(n.child) {
		return false
	}
	if tn, ok := n.child.(*termNode); ok && tn.typ == errorTerm {
		return false
	}
	return !n.child.match(line)
}

func (n *notNode) keywords() []string { return nil } // 排除项不参与高亮

// containsTimeTerm 报告子树是否含 timeTerm（NULL 传播范围判定）。
func containsTimeTerm(n queryNode) bool {
	switch v := n.(type) {
	case *termNode:
		return v.typ == timeTerm
	case *notNode:
		return containsTimeTerm(v.child)
	case *andNode:
		for _, c := range v.children {
			if containsTimeTerm(c) {
				return true
			}
		}
	case *orNode:
		for _, c := range v.children {
			if containsTimeTerm(c) {
				return true
			}
		}
	}
	return false
}

// --- Tokenizer ---

type tokenKind int

const (
	tokKeyword tokenKind = iota
	tokField
	tokAnd
	tokOr
	tokNot
	tokLParen
	tokRParen
)

type token struct {
	kind  tokenKind
	field string
	value string
}

var fieldPrefixes = []string{
	"time:",
	"traceId:", "thread:", "level:", "logger:", "message:", "source:",
}

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

// isOperatorOrField reports whether tok is an AND/OR operator or a field: prefix,
// i.e. it must NOT be swallowed as the value of a preceding "field:" token.
func isOperatorOrField(tok string) bool {
	switch strings.ToUpper(tok) {
	case "AND", "OR":
		return true
	}
	for _, prefix := range fieldPrefixes {
		if strings.HasPrefix(tok, prefix) {
			return true
		}
	}
	return false
}

// --- Recursive descent parser ---
// Precedence: AND > OR, adjacent terms = implicit AND
// anchor 为 time: 条件的锚定时刻（UTC），测试可注入固定值。

func parseOrExpr(tokens []token, pos int, anchor time.Time) (queryNode, int) {
	left, pos := parseAndExpr(tokens, pos, anchor)
	children := []queryNode{left}
	for pos < len(tokens) && tokens[pos].kind == tokOr {
		pos++ // consume OR
		right, newPos := parseAndExpr(tokens, pos, anchor)
		children = append(children, right)
		pos = newPos
	}
	if len(children) == 1 {
		return children[0], pos
	}
	return &orNode{children: children}, pos
}

func parseAndExpr(tokens []token, pos int, anchor time.Time) (queryNode, int) {
	left, pos := parseNotExpr(tokens, pos, anchor)
	children := []queryNode{left}
	for pos < len(tokens) {
		if tokens[pos].kind == tokOr || tokens[pos].kind == tokRParen {
			break
		}
		if tokens[pos].kind == tokAnd {
			pos++ // consume AND
		}
		// expect a term (implicit AND if adjacent terms)
		if pos >= len(tokens) || tokens[pos].kind == tokOr || tokens[pos].kind == tokRParen {
			break
		}
		next, newPos := parseNotExpr(tokens, pos, anchor)
		children = append(children, next)
		pos = newPos
	}
	if len(children) == 1 {
		return children[0], pos
	}
	return &andNode{children: children}, pos
}

func parseNotExpr(tokens []token, pos int, anchor time.Time) (queryNode, int) {
	if pos < len(tokens) && tokens[pos].kind == tokNot {
		child, newPos := parseNotExpr(tokens, pos+1, anchor) // 支持 NOT NOT
		return &notNode{child: child}, newPos
	}
	return parseTerm(tokens, pos, anchor)
}

func parseTerm(tokens []token, pos int, anchor time.Time) (queryNode, int) {
	if pos >= len(tokens) {
		return &termNode{typ: keywordTerm, value: ""}, pos
	}
	tok := tokens[pos]
	if tok.kind == tokLParen {
		node, newPos := parseOrExpr(tokens, pos+1, anchor)
		if newPos < len(tokens) && tokens[newPos].kind == tokRParen {
			newPos++ // 消费 )
		}
		return node, newPos
	}
	switch tok.kind {
	case tokField:
		n := &termNode{field: tok.field, value: tok.value}
		if tok.field == "time" {
			// time: 值白名单解析——明确意图，非法即报错（不静默降级关键字）
			t1, t2, op, err := parseTimeExpr(tok.value, anchor)
			if err != nil {
				n.typ = errorTerm
				n.errMsg = err.Error()
			} else {
				n.typ = timeTerm
				n.time, n.time2, n.op = t1, t2, op
			}
			return n, pos + 1
		}
		n.typ = fieldTerm
		return n, pos + 1
	case tokKeyword:
		return &termNode{typ: keywordTerm, value: tok.value}, pos + 1
	default:
		// AND/OR at term position, treat as literal keyword
		return &termNode{typ: keywordTerm, value: tok.value}, pos + 1
	}
}

// --- SearchQuery ---

type SearchQuery struct {
	Raw    string
	root   queryNode
	errMsg string // 任一 time: 值解析错误（整个查询匹配空集，UI 红字提示）
}

func parseSearchQuery(input string) SearchQuery {
	return parseSearchQueryAt(input, time.Now()) // 本地视角锚点（与 parser 的 Local 解析一致）
}

// parseSearchQueryAt 以指定锚定时刻解析（锚点 location 即查询值的时区视角）；
// 测试注入固定时刻获得确定性结果。
func parseSearchQueryAt(input string, anchor time.Time) SearchQuery {
	input = strings.TrimSpace(input)
	if input == "" {
		return SearchQuery{Raw: input}
	}
	tokens := tokenize(input)
	if len(tokens) == 0 {
		return SearchQuery{Raw: input}
	}
	root, _ := parseOrExpr(tokens, 0, anchor)
	q := SearchQuery{Raw: input, root: root}
	if msg := collectTimeError(root); msg != "" {
		q.errMsg = msg
	}
	return q
}

// collectTimeError 深度收集第一个 time: 解析错误（OR 右支也不能静默吞）。
func collectTimeError(n queryNode) string {
	switch v := n.(type) {
	case *termNode:
		if v.typ == errorTerm {
			return v.errMsg
		}
	case *notNode:
		return collectTimeError(v.child)
	case *andNode:
		for _, c := range v.children {
			if m := collectTimeError(c); m != "" {
				return m
			}
		}
	case *orNode:
		for _, c := range v.children {
			if m := collectTimeError(c); m != "" {
				return m
			}
		}
	}
	return ""
}

// isPartialQuery 报告 s 是否为仍在输入中的未完成查询。
// 输入操作符/括号/引号时查询处于中间态，调用方应保持上次有效结果，
// 而不是按残缺 parse 重新过滤（否则 and/or/not 会被当字面 keyword 搜，结果突变）。
func isPartialQuery(s string) bool {
	t := strings.TrimRight(s, " \t")
	low := strings.ToLower(t)
	switch {
	case low == "and" || low == "or" || low == "not":
		return true
	case strings.HasSuffix(low, " and") || strings.HasSuffix(low, " or") || strings.HasSuffix(low, " not"):
		return true
	case strings.HasSuffix(t, "("):
		return true
	case strings.Count(s, "(") > strings.Count(s, ")"):
		return true
	case strings.Count(s, `"`)%2 != 0:
		return true
	}
	return isPartialTimeValue(t)
}

// isPartialTimeValue 报告末尾 time: 词是否仍在输入中：
// 值解析失败但只含合法字符（数字/分隔符/方向符/时长单位）→ 视为未完成，保持上次结果。
// 含其它字母 → 确定错误，走 errorTerm 红字提示。
func isPartialTimeValue(t string) bool {
	idx := strings.LastIndex(t, "time:")
	if idx < 0 {
		return false
	}
	rest := t[idx+len("time:"):]
	if rest == "" {
		return true
	}
	if _, _, _, err := parseTimeExpr(rest, time.Now()); err == nil {
		return false // 完整合法值不是中间态
	}
	for _, r := range rest {
		if !(r >= '0' && r <= '9') && !strings.ContainsRune(":.-_Ttsmhd><", r) {
			return false
		}
	}
	return true
}

// strippedQuery 去掉查询末尾未完成的操作符（and/or/not）与 time: 悬空词，
// 用于中间态时用"剩余完整部分"搜索（如 "ERROR not" → "ERROR"，"ERROR time:>" → "ERROR"）。
func strippedQuery(s string) string {
	t := strings.TrimRight(s, " \t")
	low := strings.ToLower(t)
	for _, op := range []string{" and", " or", " not"} {
		if strings.HasSuffix(low, op) {
			return strings.TrimRight(t[:len(t)-len(op)], " \t")
		}
	}
	if low == "and" || low == "or" || low == "not" {
		return ""
	}
	// time: 悬空词剥离：中间态按剩余完整部分过滤，结果不突变
	if isPartialTimeValue(t) {
		if idx := strings.LastIndex(t, "time:"); idx >= 0 {
			return strings.TrimRight(t[:idx], " \t")
		}
	}
	return s
}

func (q SearchQuery) MatchLine(line *model.ParsedLine) bool {
	if q.root == nil {
		return true
	}
	if q.errMsg != "" {
		return false // time: 解析错误 → 空集，不降级
	}
	return q.root.match(line)
}

func (q SearchQuery) IsEmpty() bool {
	return q.root == nil
}

// ParseError 返回 time: 解析错误信息（无错为空），UI 用红字展示。
func (q SearchQuery) ParseError() string {
	return q.errMsg
}

func (q SearchQuery) HighlightKeywords() []string {
	if q.root == nil {
		return nil
	}
	return q.root.keywords()
}

// TimeRangeHint 显示实际解析出的绝对时刻（含日期；相对时间附原始写法）。
// OR/NOT 语境不显示（时间条件不构成过滤窗口）。
func (q SearchQuery) TimeRangeHint() string {
	if q.errMsg != "" {
		return ""
	}
	after, before, rel := findTimeRange(q.root)
	if after == nil && before == nil {
		return ""
	}
	const layout = "01-02 15:04:05"
	switch {
	case after != nil && before != nil:
		return after.Format(layout) + "~" + before.Format(layout)
	case after != nil:
		// 锚点 location 即展示视角（生产为本地，测试随注入的 anchor）
		h := "> " + after.Format(layout)
		if rel != "" {
			h += " (" + rel + ")"
		}
		return h
	default:
		return "< " + before.Format(layout)
	}
}

// findTimeRange 收集 AND 语境下的时间窗口；返回 (下界, 上界, 相对写法)。
func findTimeRange(n queryNode) (*time.Time, *time.Time, string) {
	if n == nil {
		return nil, nil, ""
	}
	switch v := n.(type) {
	case *termNode:
		if v.typ != timeTerm {
			return nil, nil, ""
		}
		if v.time2 != nil { // 区间糖
			return v.time, v.time2, ""
		}
		switch v.op {
		case ">", ">=":
			if rel := strings.TrimLeft(v.value, "><="); strings.HasPrefix(rel, "-") {
				return v.time, nil, rel // 相对写法（如 -10m），hint 附注
			}
			return v.time, nil, ""
		case "<", "<=":
			return nil, v.time, ""
		}
		return nil, nil, ""
	case *andNode:
		var after, before *time.Time
		rel := ""
		for _, c := range v.children {
			a, b, r := findTimeRange(c)
			if a != nil {
				after = a
				if r != "" {
					rel = r
				}
			}
			if b != nil {
				before = b
			}
		}
		return after, before, rel
	}
	return nil, nil, "" // orNode/notNode：不构成过滤窗口
}

// --- time: 值解析（词法白名单，唯一形态映射，零试探） ---

var (
	timeClockRe = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}))?$`)                              // 9:00 / 09:00:30
	timeDateRe  = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)                                    // 2026-08-26
	timeDTRe    = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})[T_](\d{1,2}):(\d{2})(?::(\d{2}))?$`)   // 2026-08-26T09:00[:30]（_ 宽容别名）
	relSegRe    = regexp.MustCompile(`\d+(?:\.\d+)?[smhd]`)                                          // 相对时长段：10m / 1h30m / 2d
)

// parseTimeExpr 解析 time: 的值：方向符+时间值 / 相对时长（默认 >）/ 闭区间 a..b。
// 返回（下界时刻, 区间上界, 操作符, 错误）；一切时刻均为 UTC。
func parseTimeExpr(raw string, anchor time.Time) (*time.Time, *time.Time, string, error) {
	if raw == "" {
		return nil, nil, "", fmt.Errorf("缺少时间值")
	}
	// 区间糖：a..b（闭区间，无方向符）
	if i := strings.Index(raw, ".."); i >= 0 {
		lo, err := parseTimeValue(raw[:i], anchor)
		if err != nil {
			return nil, nil, "", err
		}
		hi, err := parseTimeValue(raw[i+2:], anchor)
		if err != nil {
			return nil, nil, "", fmt.Errorf("区间上界: %w", err)
		}
		if lo.After(*hi) {
			lo, hi = hi, lo
		}
		return lo, hi, "..", nil
	}
	// 方向符（最长匹配）
	op, rest := "", raw
	for _, cand := range []string{">=", "<=", ">", "<"} {
		if strings.HasPrefix(rest, cand) {
			op, rest = cand, rest[len(cand):]
			break
		}
	}
	// 相对时长：-10m / -1h30m（可带显式方向符，默认 >）
	if strings.HasPrefix(rest, "-") {
		d, err := parseRelDuration(rest[1:])
		if err != nil {
			return nil, nil, "", err
		}
		if op == "" {
			op = ">"
		}
		t := anchor.Add(-d)
		return &t, nil, op, nil
	}
	if op == "" {
		return nil, nil, "", fmt.Errorf("缺少比较符（> < >= <=）或区间（..）")
	}
	t, err := parseTimeValue(rest, anchor)
	if err != nil {
		return nil, nil, "", err
	}
	return t, nil, op, nil
}

// parseTimeValue 解析单个时间值：时分秒（锚定 anchor 当天）/ 日期 / 日期T时间。一律 UTC。
func parseTimeValue(s string, anchor time.Time) (*time.Time, error) {
	if m := timeClockRe.FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		sec := 0
		if m[3] != "" {
			sec, _ = strconv.Atoi(m[3])
		}
		if h > 23 || mi > 59 || sec > 59 {
			return nil, fmt.Errorf("时间越界: %s", s)
		}
		t := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), h, mi, sec, 0, anchor.Location())
		return &t, nil
	}
	if m := timeDateRe.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, anchor.Location())
		if t.Month() != time.Month(mo) {
			return nil, fmt.Errorf("日期非法: %s", s) // 2026-02-30 溢出
		}
		return &t, nil
	}
	if m := timeDTRe.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		h, _ := strconv.Atoi(m[4])
		mi, _ := strconv.Atoi(m[5])
		sec := 0
		if m[6] != "" {
			sec, _ = strconv.Atoi(m[6])
		}
		if h > 23 || mi > 59 || sec > 59 {
			return nil, fmt.Errorf("时间越界: %s", s)
		}
		t := time.Date(y, time.Month(mo), d, h, mi, sec, 0, anchor.Location())
		if t.Month() != time.Month(mo) {
			return nil, fmt.Errorf("日期非法: %s", s)
		}
		return &t, nil
	}
	return nil, fmt.Errorf("无法识别时间值 %q（支持 H:MM[:SS] / YYYY-MM-DD / YYYY-MM-DD THH:MM[:SS] / -时长）", s)
}

// parseRelDuration 相对时长：s/m/h/d 单位可复合（1h30m、90m、2d），d = 24h；拒绝正号与 ns/us/ms。
func parseRelDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("缺少时长")
	}
	segs := relSegRe.FindAllString(s, -1)
	if len(segs) == 0 || strings.Join(segs, "") != s {
		return 0, fmt.Errorf("无法识别时长 %q（支持 s/m/h/d 复合，如 1h30m）", s)
	}
	var total time.Duration
	for _, seg := range segs {
		num, _ := strconv.ParseFloat(seg[:len(seg)-1], 64)
		switch seg[len(seg)-1] {
		case 'd':
			total += time.Duration(num * 24 * float64(time.Hour))
		case 'h':
			total += time.Duration(num * float64(time.Hour))
		case 'm':
			total += time.Duration(num * float64(time.Minute))
		case 's':
			total += time.Duration(num * float64(time.Second))
		}
	}
	return total, nil
}
