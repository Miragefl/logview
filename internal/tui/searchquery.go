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
		return containsIgnoreCase(line.Message, n.value) ||
			containsIgnoreCase(line.Raw.Text, n.value) ||
			containsIgnoreCase(line.TraceID, n.value) ||
			containsIgnoreCase(line.Thread, n.value) ||
			containsIgnoreCase(line.Logger, n.value) ||
			containsIgnoreCase(line.Level, n.value)
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

// --- Tokenizer ---

type tokenKind int

const (
	tokKeyword tokenKind = iota
	tokField
	tokAnd
	tokOr
)

type token struct {
	kind  tokenKind
	field string
	value string
}

var fieldPrefixes = []string{
	"after:", "before:",
	"traceId:", "thread:", "level:", "logger:", "message:", "source:", "time:",
}

func tokenize(input string) []token {
	var tokens []token
	for _, tok := range strings.Fields(input) {
		if tok == "AND" {
			tokens = append(tokens, token{kind: tokAnd, value: "AND"})
			continue
		}
		if tok == "OR" {
			tokens = append(tokens, token{kind: tokOr, value: "OR"})
			continue
		}
		matched := false
		for _, prefix := range fieldPrefixes {
			if strings.HasPrefix(tok, prefix) {
				field := strings.TrimSuffix(prefix, ":")
				value := tok[len(prefix):]
				tokens = append(tokens, token{kind: tokField, field: field, value: value})
				matched = true
				break
			}
		}
		if !matched {
			tokens = append(tokens, token{kind: tokKeyword, value: tok})
		}
	}
	return tokens
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
	left, pos := parseTerm(tokens, pos)
	children := []queryNode{left}
	for pos < len(tokens) {
		if tokens[pos].kind == tokOr {
			break
		}
		if tokens[pos].kind == tokAnd {
			pos++ // consume AND
		}
		// expect a term (implicit AND if adjacent terms)
		if pos >= len(tokens) || tokens[pos].kind == tokOr {
			break
		}
		next, newPos := parseTerm(tokens, pos)
		children = append(children, next)
		pos = newPos
	}
	if len(children) == 1 {
		return children[0], pos
	}
	return &andNode{children: children}, pos
}

func parseTerm(tokens []token, pos int) (queryNode, int) {
	if pos >= len(tokens) {
		return &termNode{typ: keywordTerm, value: ""}, pos
	}
	tok := tokens[pos]
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
