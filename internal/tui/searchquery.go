package tui

import (
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
	timeAfterTerm
	timeBeforeTerm
)

type termNode struct {
	typ   termType
	field string
	value string
	time  *time.Time
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
	case timeAfterTerm:
		if line.Time.IsZero() || n.time == nil {
			return false
		}
		lineMin := line.Time.Hour()*60 + line.Time.Minute()
		afterMin := n.time.Hour()*60 + n.time.Minute()
		return lineMin > afterMin
	case timeBeforeTerm:
		if line.Time.IsZero() || n.time == nil {
			return false
		}
		lineMin := line.Time.Hour()*60 + line.Time.Minute()
		beforeMin := n.time.Hour()*60 + n.time.Minute()
		return lineMin < beforeMin
	}
	return true
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

func (n *notNode) match(line *model.ParsedLine) bool { return !n.child.match(line) }

func (n *notNode) keywords() []string { return nil } // 排除项不参与高亮

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
	"after:", "before:",
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

func parseOrExpr(tokens []token, pos int) (queryNode, int) {
	left, pos := parseAndExpr(tokens, pos)
	children := []queryNode{left}
	for pos < len(tokens) && tokens[pos].kind == tokOr {
		pos++ // consume OR
		right, newPos := parseAndExpr(tokens, pos)
		children = append(children, right)
		pos = newPos
	}
	if len(children) == 1 {
		return children[0], pos
	}
	return &orNode{children: children}, pos
}

func parseAndExpr(tokens []token, pos int) (queryNode, int) {
	left, pos := parseNotExpr(tokens, pos)
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
		next, newPos := parseNotExpr(tokens, pos)
		children = append(children, next)
		pos = newPos
	}
	if len(children) == 1 {
		return children[0], pos
	}
	return &andNode{children: children}, pos
}

func parseNotExpr(tokens []token, pos int) (queryNode, int) {
	if pos < len(tokens) && tokens[pos].kind == tokNot {
		child, newPos := parseNotExpr(tokens, pos+1) // 支持 NOT NOT
		return &notNode{child: child}, newPos
	}
	return parseTerm(tokens, pos)
}

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
	case tokField:
		n := &termNode{field: tok.field, value: tok.value}
		switch tok.field {
		case "after":
			t, err := time.Parse("15:04", tok.value)
			if err == nil {
				n.typ = timeAfterTerm
				pt := t
				n.time = &pt
			} else {
				n.typ = keywordTerm
				n.value = tok.field + ":" + tok.value
			}
		case "before":
			t, err := time.Parse("15:04", tok.value)
			if err == nil {
				n.typ = timeBeforeTerm
				pt := t
				n.time = &pt
			} else {
				n.typ = keywordTerm
				n.value = tok.field + ":" + tok.value
			}
		default:
			n.typ = fieldTerm
		}
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
	Raw  string
	root queryNode
}

func parseSearchQuery(input string) SearchQuery {
	input = strings.TrimSpace(input)
	if input == "" {
		return SearchQuery{Raw: input}
	}
	tokens := tokenize(input)
	if len(tokens) == 0 {
		return SearchQuery{Raw: input}
	}
	root, _ := parseOrExpr(tokens, 0)
	return SearchQuery{Raw: input, root: root}
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
	return false
}

// strippedQuery 去掉查询末尾未完成的操作符（and/or/not），
// 用于中间态时用"剩余完整部分"搜索（如 "ERROR not" → "ERROR"，"not" → ""）。
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
	return s
}

func (q SearchQuery) MatchLine(line *model.ParsedLine) bool {
	if q.root == nil {
		return true
	}
	return q.root.match(line)
}

func (q SearchQuery) IsEmpty() bool {
	return q.root == nil
}

func (q SearchQuery) HighlightKeywords() []string {
	if q.root == nil {
		return nil
	}
	return q.root.keywords()
}

func (q SearchQuery) TimeRangeHint() string {
	after, before := findTimeRange(q.root)
	if after == nil && before == nil {
		return ""
	}
	a, b := "", ""
	if after != nil {
		a = after.Format("15:04")
	}
	if before != nil {
		b = before.Format("15:04")
	}
	return a + "~" + b
}

func findTimeRange(n queryNode) (*time.Time, *time.Time) {
	if n == nil {
		return nil, nil
	}
	switch v := n.(type) {
	case *termNode:
		var after, before *time.Time
		if v.typ == timeAfterTerm {
			after = v.time
		}
		if v.typ == timeBeforeTerm {
			before = v.time
		}
		return after, before
	case *andNode:
		var after, before *time.Time
		for _, c := range v.children {
			a, b := findTimeRange(c)
			if a != nil {
				after = a
			}
			if b != nil {
				before = b
			}
		}
		return after, before
	case *orNode:
		if len(v.children) > 0 {
			return findTimeRange(v.children[0])
		}
	}
	return nil, nil
}
