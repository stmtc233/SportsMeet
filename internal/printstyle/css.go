package printstyle

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxCSSBytes is the most custom CSS a style may hold.
const MaxCSSBytes = 256 << 10

// maxWarnings bounds the list a hostile input could make long; one more
// line says how many were left out.
const maxWarnings = 40

// maxCSSDepth is how deeply blocks may nest.
const maxCSSDepth = 24

// SanitizeCSS removes from a stylesheet everything that could load or run
// something from outside: @import and the other at-rules it does not know,
// url() and image-set() and the like that point anywhere but an embedded
// png / jpeg / webp image, an embedded font or an asset: reference,
// expression(), behavior, -moz-binding, javascript: and a style element's
// end tag. It reads the text the way a browser does (CSS Syntax: tokens
// first, escapes decoded), so u\72l( is the url function it is. Each
// removal is a warning in Chinese. A stylesheet over MaxCSSBytes is an
// error.
func SanitizeCSS(src string) (clean string, warnings []string, err error) {
	if len(src) > MaxCSSBytes {
		return "", nil, fmt.Errorf("自定义 CSS 超过 %d KB", MaxCSSBytes>>10)
	}
	w := &warner{}
	s := &cssSanitizer{w: w}
	clean = s.sheet(parseCSS(tokenizeCSS(src)), false, 0)
	// A second line behind the token checks: whatever happened, nothing in
	// the output may end the style element.
	if low := strings.ToLower(clean); strings.Contains(low, "</style") || strings.Contains(low, "<script") {
		return "", w.list(), errors.New("CSS 含有不允许的内容")
	}
	return clean, w.list(), nil
}

// SanitizeDeclarations cleans the value of a style attribute: a list of
// declarations, nothing else.
func SanitizeDeclarations(src string) (string, []string) {
	if len(src) > MaxCSSBytes {
		return "", []string{"style 属性太长，已删除"}
	}
	w := &warner{}
	s := &cssSanitizer{w: w}
	nodes := parseCSS(tokenizeCSS(src))
	var kept []cssNode
	for _, n := range nodes {
		if n.kind != nodeDecl {
			w.add("style 属性里的规则已删除")
			continue
		}
		kept = append(kept, n)
	}
	out := s.sheet(kept, false, 1)
	if low := strings.ToLower(out); strings.Contains(low, "</style") || strings.Contains(low, "<script") {
		return "", w.list()
	}
	return out, w.list()
}

type warner struct {
	msgs []string
	more int
}

func (w *warner) add(format string, args ...any) {
	if len(w.msgs) >= maxWarnings {
		w.more++
		return
	}
	w.msgs = append(w.msgs, fmt.Sprintf(format, args...))
}

func (w *warner) list() []string {
	if w.more > 0 {
		return append(w.msgs, fmt.Sprintf("还有 %d 处已删除", w.more))
	}
	return w.msgs
}

// ---- tokens ----

type tokenKind int

const (
	tkSpace tokenKind = iota
	tkIdent
	tkFunction
	tkAtKeyword
	tkHash
	tkString
	tkBadString
	tkURL
	tkBadURL
	tkNumber
	tkDelim
	tkColon
	tkSemicolon
	tkComma
	tkLBrace
	tkRBrace
	tkLParen
	tkRParen
	tkLBracket
	tkRBracket
)

type cssToken struct {
	kind tokenKind
	raw  string // as written; what is printed back
	val  string // decoded: an ident's name, a string's or url's content
}

type cssLexer struct {
	s   string
	pos int
}

func tokenizeCSS(src string) []cssToken {
	// A leading BOM and NULs are noise; NUL would also hide words from checks.
	src = strings.ReplaceAll(strings.TrimPrefix(src, "\uFEFF"), "\x00", "\uFFFD")
	l := &cssLexer{s: src}
	var out []cssToken
	for l.pos < len(l.s) {
		if t, ok := l.next(); ok {
			out = append(out, t)
		}
	}
	return out
}

func (l *cssLexer) peek(n int) rune {
	if l.pos+n >= len(l.s) {
		return -1
	}
	// Only ASCII is looked at through peek; multi-byte runes read as high.
	b := l.s[l.pos+n]
	if b >= 0x80 {
		return 0x80
	}
	return rune(b)
}

func isNewline(r rune) bool { return r == '\n' || r == '\r' || r == '\f' }
func isSpace(r rune) bool   { return r == ' ' || r == '\t' || isNewline(r) }
func isDigit(r rune) bool   { return r >= '0' && r <= '9' }
func isHex(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
func isNameStart(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || r >= 0x80
}
func isNameChar(r rune) bool { return isNameStart(r) || isDigit(r) || r == '-' }

// validEscape: a backslash not before a newline (or the end).
func (l *cssLexer) validEscapeAt(n int) bool {
	return l.peek(n) == '\\' && l.peek(n+1) != -1 && !isNewline(l.peek(n+1))
}

func (l *cssLexer) identStartAt(n int) bool {
	c := l.peek(n)
	switch {
	case c == '-':
		c1 := l.peek(n + 1)
		return isNameStart(c1) || c1 == '-' || l.validEscapeAt(n+1)
	case isNameStart(c):
		return true
	}
	return l.validEscapeAt(n)
}

// escape reads the escape at pos (a backslash) and returns what it means.
func (l *cssLexer) escape() string {
	l.pos++ // the backslash
	if l.pos >= len(l.s) {
		return "\uFFFD"
	}
	if c := l.peek(0); isHex(c) {
		n := 0
		v := 0
		for n < 6 && l.pos < len(l.s) && isHex(rune(l.s[l.pos])) {
			d := l.s[l.pos]
			switch {
			case d >= 'a':
				v = v*16 + int(d-'a') + 10
			case d >= 'A':
				v = v*16 + int(d-'A') + 10
			default:
				v = v*16 + int(d-'0')
			}
			l.pos++
			n++
		}
		if l.pos < len(l.s) && isSpace(rune(l.s[l.pos])) {
			if l.s[l.pos] == '\r' && l.pos+1 < len(l.s) && l.s[l.pos+1] == '\n' {
				l.pos++
			}
			l.pos++
		}
		if v == 0 || v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF) {
			return "\uFFFD"
		}
		return string(rune(v))
	}
	r, size := utf8.DecodeRuneInString(l.s[l.pos:])
	l.pos += size
	return string(r)
}

// name reads an identifier's characters, decoding escapes.
func (l *cssLexer) name() string {
	var b strings.Builder
	for l.pos < len(l.s) {
		c := l.peek(0)
		switch {
		case isNameChar(c):
			r, size := utf8.DecodeRuneInString(l.s[l.pos:])
			b.WriteRune(r)
			l.pos += size
		case l.validEscapeAt(0):
			b.WriteString(l.escape())
		default:
			return b.String()
		}
	}
	return b.String()
}

func (l *cssLexer) next() (cssToken, bool) {
	start := l.pos
	c := l.peek(0)
	tok := func(k tokenKind, val string) (cssToken, bool) {
		return cssToken{kind: k, raw: l.s[start:l.pos], val: val}, true
	}
	switch {
	case isSpace(c):
		for l.pos < len(l.s) && isSpace(rune(l.s[l.pos])) {
			l.pos++
		}
		return cssToken{kind: tkSpace, raw: " "}, true
	case c == '/' && l.peek(1) == '*':
		end := strings.Index(l.s[l.pos+2:], "*/")
		if end < 0 {
			l.pos = len(l.s)
		} else {
			l.pos += 2 + end + 2
		}
		return cssToken{kind: tkSpace, raw: " "}, true
	case c == '"' || c == '\'':
		return l.str(c)
	case c == '#':
		if isNameChar(l.peek(1)) || l.validEscapeAt(1) {
			l.pos++
			return tok(tkHash, l.name())
		}
	case c == '<' && strings.HasPrefix(l.s[l.pos:], "<!--"):
		l.pos += 4
		return cssToken{kind: tkSpace, raw: " "}, true
	case c == '-' && strings.HasPrefix(l.s[l.pos:], "-->"):
		l.pos += 3
		return cssToken{kind: tkSpace, raw: " "}, true
	case c == '@':
		if l.identStartAt(1) {
			l.pos++
			return tok(tkAtKeyword, l.name())
		}
	case c == '\\':
		if l.validEscapeAt(0) {
			return l.identLike(start)
		}
	case isDigit(c) || (c == '.' && isDigit(l.peek(1))) || ((c == '+' || c == '-') && (isDigit(l.peek(1)) || (l.peek(1) == '.' && isDigit(l.peek(2))))):
		return l.number(start)
	case isNameStart(c) || (c == '-' && l.identStartAt(0)):
		return l.identLike(start)
	case c == ':':
		l.pos++
		return tok(tkColon, "")
	case c == ';':
		l.pos++
		return tok(tkSemicolon, "")
	case c == ',':
		l.pos++
		return tok(tkComma, "")
	case c == '{':
		l.pos++
		return tok(tkLBrace, "")
	case c == '}':
		l.pos++
		return tok(tkRBrace, "")
	case c == '(':
		l.pos++
		return tok(tkLParen, "")
	case c == ')':
		l.pos++
		return tok(tkRParen, "")
	case c == '[':
		l.pos++
		return tok(tkLBracket, "")
	case c == ']':
		l.pos++
		return tok(tkRBracket, "")
	}
	r, size := utf8.DecodeRuneInString(l.s[l.pos:])
	l.pos += size
	return cssToken{kind: tkDelim, raw: l.s[start:l.pos], val: string(r)}, true
}

func (l *cssLexer) number(start int) (cssToken, bool) {
	if c := l.peek(0); c == '+' || c == '-' {
		l.pos++
	}
	for isDigit(l.peek(0)) {
		l.pos++
	}
	if l.peek(0) == '.' && isDigit(l.peek(1)) {
		l.pos++
		for isDigit(l.peek(0)) {
			l.pos++
		}
	}
	if c := l.peek(0); c == 'e' || c == 'E' {
		n := 1
		if s := l.peek(1); s == '+' || s == '-' {
			n = 2
		}
		if isDigit(l.peek(n)) {
			l.pos += n
			for isDigit(l.peek(0)) {
				l.pos++
			}
		}
	}
	switch {
	case l.identStartAt(0):
		l.name()
	case l.peek(0) == '%':
		l.pos++
	}
	return cssToken{kind: tkNumber, raw: l.s[start:l.pos]}, true
}

func (l *cssLexer) str(quote rune) (cssToken, bool) {
	start := l.pos
	l.pos++
	var b strings.Builder
	for l.pos < len(l.s) {
		c := l.peek(0)
		switch {
		case c == quote:
			l.pos++
			return cssToken{kind: tkString, raw: l.s[start:l.pos], val: b.String()}, true
		case isNewline(c):
			return cssToken{kind: tkBadString, raw: l.s[start:l.pos]}, true
		case c == '\\':
			if l.pos+1 >= len(l.s) {
				l.pos++
				return cssToken{kind: tkBadString, raw: l.s[start:l.pos]}, true
			}
			if isNewline(l.peek(1)) {
				l.pos += 2 // a line continuation
				continue
			}
			b.WriteString(l.escape())
		default:
			r, size := utf8.DecodeRuneInString(l.s[l.pos:])
			b.WriteRune(r)
			l.pos += size
		}
	}
	// Unterminated at the end: dropped like a bad string, not completed.
	return cssToken{kind: tkBadString, raw: l.s[start:]}, true
}

func (l *cssLexer) identLike(start int) (cssToken, bool) {
	name := l.name()
	if l.peek(0) != '(' {
		return cssToken{kind: tkIdent, raw: l.s[start:l.pos], val: name}, true
	}
	l.pos++
	if !strings.EqualFold(name, "url") {
		return cssToken{kind: tkFunction, raw: l.s[start:l.pos], val: name}, true
	}
	// url( followed by a string is the function; anything else is a url token.
	save := l.pos
	for l.pos < len(l.s) && isSpace(rune(l.s[l.pos])) {
		l.pos++
	}
	if c := l.peek(0); c == '"' || c == '\'' {
		l.pos = save
		return cssToken{kind: tkFunction, raw: l.s[start:l.pos], val: name}, true
	}
	var b strings.Builder
	for l.pos < len(l.s) {
		c := l.peek(0)
		switch {
		case c == ')':
			l.pos++
			return cssToken{kind: tkURL, raw: l.s[start:l.pos], val: b.String()}, true
		case isSpace(c):
			for l.pos < len(l.s) && isSpace(rune(l.s[l.pos])) {
				l.pos++
			}
			if l.pos >= len(l.s) {
				return cssToken{kind: tkBadURL, raw: l.s[start:]}, true
			}
			if l.peek(0) == ')' {
				l.pos++
				return cssToken{kind: tkURL, raw: l.s[start:l.pos], val: b.String()}, true
			}
			return l.badURL(start)
		case c == '"' || c == '\'' || c == '(' || c < 0x20 && c >= 0 || c == 0x7f:
			return l.badURL(start)
		case c == '\\':
			if !l.validEscapeAt(0) {
				return l.badURL(start)
			}
			b.WriteString(l.escape())
		default:
			r, size := utf8.DecodeRuneInString(l.s[l.pos:])
			b.WriteRune(r)
			l.pos += size
		}
	}
	return cssToken{kind: tkBadURL, raw: l.s[start:]}, true
}

func (l *cssLexer) badURL(start int) (cssToken, bool) {
	for l.pos < len(l.s) {
		c := l.peek(0)
		if c == ')' {
			l.pos++
			break
		}
		if l.validEscapeAt(0) {
			l.escape()
			continue
		}
		_, size := utf8.DecodeRuneInString(l.s[l.pos:])
		l.pos += size
	}
	return cssToken{kind: tkBadURL, raw: l.s[start:l.pos]}, true
}

// ---- structure ----

type nodeKind int

const (
	nodeDecl nodeKind = iota // tokens up to ; or the end of the block
	nodeRule                 // prelude { block }
	nodeAt                   // @name prelude ; or @name prelude { block }
)

type cssNode struct {
	kind     nodeKind
	prelude  []cssToken // for an at-rule, starting with the at-keyword
	block    []cssNode
	hasBlock bool
}

type cssParser struct {
	toks  []cssToken
	pos   int
	depth int
}

func parseCSS(toks []cssToken) []cssNode {
	p := &cssParser{toks: toks}
	return p.items(true)
}

// items reads the contents of a block (or the sheet) up to its closing brace.
func (p *cssParser) items(top bool) []cssNode {
	var out []cssNode
	for p.pos < len(p.toks) {
		t := p.toks[p.pos]
		switch t.kind {
		case tkSpace, tkSemicolon:
			p.pos++
			continue
		case tkRBrace:
			p.pos++
			if top {
				continue // a stray brace at the top
			}
			return out
		}
		out = append(out, p.item())
	}
	return out
}

// item reads one declaration, rule or at-rule. Brackets and parentheses
// nest: a ; or { inside them does not end the item.
func (p *cssParser) item() cssNode {
	n := cssNode{kind: nodeDecl}
	if p.toks[p.pos].kind == tkAtKeyword {
		n.kind = nodeAt
	}
	var stack []tokenKind
loop:
	for p.pos < len(p.toks) {
		t := p.toks[p.pos]
		switch t.kind {
		case tkFunction, tkLParen:
			stack = append(stack, tkRParen)
		case tkLBracket:
			stack = append(stack, tkRBracket)
		case tkRParen, tkRBracket:
			if len(stack) > 0 && stack[len(stack)-1] == t.kind {
				stack = stack[:len(stack)-1]
			}
		case tkSemicolon:
			if len(stack) == 0 {
				p.pos++
				break loop
			}
		case tkRBrace:
			if len(stack) == 0 {
				break loop // the enclosing block ends; its reader consumes the brace
			}
		case tkLBrace:
			if len(stack) == 0 {
				p.pos++
				if n.kind == nodeDecl {
					n.kind = nodeRule
				}
				n.hasBlock = true
				p.depth++
				if p.depth > maxCSSDepth {
					// Too deep to keep: skip to the matching brace.
					p.skipBlock()
					n.block = nil
				} else {
					n.block = p.items(false)
				}
				p.depth--
				break loop
			}
		}
		n.prelude = append(n.prelude, t)
		p.pos++
	}
	return n
}

func (p *cssParser) skipBlock() {
	depth := 1
	for p.pos < len(p.toks) && depth > 0 {
		switch p.toks[p.pos].kind {
		case tkLBrace:
			depth++
		case tkRBrace:
			depth--
		}
		p.pos++
	}
}

// ---- checks ----

var atRules = map[string]bool{
	"media": true, "supports": true, "page": true, "font-face": true,
	"keyframes": true, "-webkit-keyframes": true, "layer": true, "container": true,
	"counter-style": true,
}

// urlFunctions take a URL or a string that is one.
var urlFunctions = map[string]bool{
	"url": true, "src": true, "image": true, "image-set": true, "-webkit-image-set": true,
	"cross-fade": true, "-webkit-cross-fade": true, "-moz-image-rect": true,
}

var (
	embeddedImage = regexp.MustCompile(`^data:image/(png|jpeg|webp)(;[a-z0-9=.+-]+)*,`)
	embeddedFont  = regexp.MustCompile(`^data:font/[a-z0-9.+-]+(;[a-z0-9=.+-]+)*,`)
)

// bannedText lists what no declaration or prelude may spell, read after
// decoding escapes and dropping white space and comments.
var bannedText = []string{"javascript:", "vbscript:", "</style", "<script", "<!--", "expression(", "-moz-binding", "behavior:"}

// urlText is how a browser reads a URL before looking at it: tabs and
// newlines gone, the ends trimmed.
func urlText(v string) string {
	v = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, v)
	return strings.TrimFunc(v, func(r rune) bool { return r <= 0x20 })
}

// AllowedImageURL reports whether a URL may be used as a picture: an
// embedded png, jpeg or webp, or an asset: reference to a style's or a
// meet's image.
func AllowedImageURL(v string) bool {
	v = urlText(v)
	if embeddedImage.MatchString(strings.ToLower(v)) {
		return true
	}
	return assetRef(v) != ""
}

// assetRef is the name in an asset: URL, "" when it is not one.
func assetRef(v string) string {
	name, ok := strings.CutPrefix(v, "asset:")
	if !ok || name == "" || strings.ContainsAny(name, " \t\r\n\f\"'()<>\\") {
		return ""
	}
	return name
}

func allowedCSSURL(v string, fontFace bool) bool {
	v = urlText(v)
	low := strings.ToLower(v)
	if embeddedFont.MatchString(low) {
		return true
	}
	if fontFace {
		return false
	}
	return embeddedImage.MatchString(low) || assetRef(v) != ""
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}

// unsafeReason says why a run of tokens must go, "" when it may stay.
func unsafeReason(toks []cssToken, fontFace bool) string {
	var text strings.Builder
	var stack []bool // open functions and parentheses: is it a URL function
	for _, t := range toks {
		switch t.kind {
		case tkSpace:
			continue
		case tkBadString, tkBadURL:
			return "格式不完整的字符串或地址"
		case tkDelim:
			if t.val == "\\" {
				// A lone backslash would join the character printed after it.
				return "多余的反斜杠"
			}
			text.WriteString(strings.ToLower(t.raw))
		case tkAtKeyword:
			// An at-rule starts its own statement; one anywhere else is not
			// something a browser would run, and not something to keep.
			return "多余的 @" + t.val
		case tkFunction:
			low := strings.ToLower(t.val)
			stack = append(stack, urlFunctions[low])
			text.WriteString(low + "(")
		case tkLParen:
			stack = append(stack, false)
			text.WriteString("(")
		case tkRParen:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			text.WriteString(")")
		case tkURL:
			if !allowedCSSURL(t.val, fontFace) {
				return "外部引用 url(" + clip(t.val) + ")"
			}
			text.WriteString(strings.ToLower(t.val))
		case tkString:
			for _, u := range stack {
				if u {
					if !allowedCSSURL(t.val, fontFace) {
						return "外部引用 " + clip(t.val)
					}
					break
				}
			}
			text.WriteString(strings.ToLower(t.val))
		case tkIdent, tkHash:
			text.WriteString(strings.ToLower(t.val))
		default:
			text.WriteString(strings.ToLower(t.raw))
		}
	}
	joined := text.String()
	for _, b := range bannedText {
		if strings.Contains(joined, b) {
			return "不允许的内容 " + strings.TrimSuffix(b, ":")
		}
	}
	for _, t := range toks {
		if low := strings.ToLower(t.raw); strings.Contains(low, "</style") || strings.Contains(low, "<script") {
			return "不允许的内容 </style"
		}
	}
	return ""
}

// raw prints tokens as written, without the white space token at either
// end (an escaped space inside an identifier is not one).
func raw(toks []cssToken) string {
	for len(toks) > 0 && toks[0].kind == tkSpace {
		toks = toks[1:]
	}
	for len(toks) > 0 && toks[len(toks)-1].kind == tkSpace {
		toks = toks[:len(toks)-1]
	}
	var b strings.Builder
	for _, t := range toks {
		b.WriteString(t.raw)
	}
	return b.String()
}

type cssSanitizer struct{ w *warner }

// sheet prints the nodes that may stay. fontFace marks a @font-face block,
// whose src may only embed fonts; depth 1 is a style attribute.
func (s *cssSanitizer) sheet(nodes []cssNode, fontFace bool, depth int) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.kind {
		case nodeDecl:
			if len(n.prelude) == 0 {
				continue
			}
			if why := unsafeReason(n.prelude, fontFace); why != "" {
				s.w.add("已删除 %s：%s", clip(raw(n.prelude)), why)
				continue
			}
			b.WriteString(raw(n.prelude))
			b.WriteString(";")
		case nodeRule:
			if why := unsafeReason(n.prelude, false); why != "" {
				s.w.add("已删除规则 %s：%s", clip(raw(n.prelude)), why)
				continue
			}
			b.WriteString(raw(n.prelude))
			b.WriteString("{")
			b.WriteString(s.sheet(n.block, false, depth+1))
			b.WriteString("}\n")
		case nodeAt:
			name := strings.ToLower(n.prelude[0].val)
			if !atRules[name] {
				s.w.add("已删除 @%s：不支持的规则", name)
				continue
			}
			if why := unsafeReason(n.prelude[1:], false); why != "" {
				s.w.add("已删除 @%s：%s", name, why)
				continue
			}
			b.WriteString(raw(n.prelude))
			if !n.hasBlock {
				b.WriteString(";\n")
				continue
			}
			b.WriteString("{")
			b.WriteString(s.sheet(n.block, name == "font-face", depth+1))
			b.WriteString("}\n")
		}
	}
	return b.String()
}
