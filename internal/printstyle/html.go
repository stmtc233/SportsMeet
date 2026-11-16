package printstyle

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// CSP is the Content-Security-Policy every report document carries as a
// second line behind the cleaning: nothing is loaded but embedded images and
// fonts, no script runs.
const CSP = "default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:"

// CSPMeta is the meta element that sets CSP.
const CSPMeta = `<meta http-equiv="Content-Security-Policy" content="` + CSP + `">`

// allowedElements are the elements a report document may hold. Everything
// else is dropped (with its content when that content is not text for the
// reader, as for script and svg) or unwrapped.
var allowedElements = setOf(
	"html", "head", "body", "title", "style", "meta",
	"header", "footer", "main", "section", "article", "div", "span", "p", "br", "hr",
	"h1", "h2", "h3", "h4", "h5", "h6", "small", "strong", "b", "em", "i", "u", "s", "del", "sub", "sup", "mark",
	"pre", "code", "blockquote", "ul", "ol", "li", "dl", "dt", "dd",
	"table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption", "colgroup", "col",
	"figure", "figcaption", "img",
)

// droppedElements go with everything inside them.
var droppedElements = setOf(
	"script", "iframe", "frame", "frameset", "object", "embed", "applet", "link", "base",
	"form", "input", "button", "select", "textarea", "option", "optgroup", "datalist", "fieldset",
	"svg", "math", "template", "noscript", "noembed", "noframes", "xmp", "plaintext",
	"audio", "video", "source", "track", "canvas", "map", "area", "picture", "portal", "dialog", "slot",
)

func setOf(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

var (
	dataAttr   = regexp.MustCompile(`^data-[a-z0-9_.-]+$`)
	countAttr  = regexp.MustCompile(`^[0-9]{1,5}$`)
	lengthAttr = regexp.MustCompile(`^[0-9]{1,5}(px|%)?$`)
)

type htmlCleaner struct {
	w          *warner
	seen       map[string]bool
	allowStyle bool
}

func (c *htmlCleaner) warn(format string, args ...any) {
	key := format
	if len(args) > 0 {
		key = format + "\x00" + args[0].(string)
	}
	if c.seen[key] {
		return
	}
	c.seen[key] = true
	c.w.add(format, args...)
}

// SanitizeDocument keeps of a whole HTML document only the elements and
// attributes a report needs (allowedElements), cleans every style element
// and style attribute with SanitizeCSS, lets an img carry only an embedded
// image or an asset: reference, and drops scripts, event attributes,
// frames, forms, svg and every link out. Only the report's own
// Content-Security-Policy meta survives. What it removes is a warning.
func SanitizeDocument(doc string) (string, []string) {
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return "", []string{"文档无法解析，已清空"}
	}
	c := &htmlCleaner{w: &warner{}, seen: map[string]bool{}, allowStyle: true}
	c.children(root)
	var buf bytes.Buffer
	if err := html.Render(&buf, root); err != nil {
		return "", append(c.w.list(), "文档无法输出，已清空")
	}
	return buf.String(), c.w.list()
}

// SanitizeFragment cleans a piece of HTML, such as a rendered Markdown page
// or a slot, by the same rules; style elements are not allowed in it.
func SanitizeFragment(frag string) (string, []string) {
	ctx := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(frag), ctx)
	if err != nil {
		return "", []string{"内容无法解析，已清空"}
	}
	holder := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	for _, n := range nodes {
		holder.AppendChild(n)
	}
	c := &htmlCleaner{w: &warner{}, seen: map[string]bool{}}
	c.children(holder)
	var buf bytes.Buffer
	for n := holder.FirstChild; n != nil; n = n.NextSibling {
		if err := html.Render(&buf, n); err != nil {
			return "", append(c.w.list(), "内容无法输出，已清空")
		}
	}
	return buf.String(), c.w.list()
}

// children cleans the children of n in place.
func (c *htmlCleaner) children(n *html.Node) {
	for ch := n.FirstChild; ch != nil; {
		next := ch.NextSibling
		c.node(n, ch)
		ch = next
	}
}

// node cleans ch, a child of parent: keeps it, removes it, or replaces it
// by its (cleaned) children.
func (c *htmlCleaner) node(parent, ch *html.Node) {
	switch ch.Type {
	case html.TextNode:
		return
	case html.DoctypeNode:
		ch.Data = "html"
		ch.Attr = nil
		return
	case html.ElementNode:
	default: // comments, raw nodes
		parent.RemoveChild(ch)
		return
	}
	tag := strings.ToLower(ch.Data)
	if ch.Namespace != "" {
		c.warn("已删除 <%s>", tag)
		parent.RemoveChild(ch)
		return
	}
	switch {
	case droppedElements[tag]:
		c.warn("已删除 <%s>", tag)
		parent.RemoveChild(ch)
	case tag == "style":
		c.style(parent, ch)
	case tag == "meta":
		c.meta(parent, ch)
	case allowedElements[tag]:
		c.attrs(ch, tag)
		if tag == "img" {
			if !hasAttr(ch, "src") {
				c.warn("已删除没有可用地址的图片")
				parent.RemoveChild(ch)
				return
			}
		}
		c.children(ch)
	default:
		// An element we do not know keeps its content: <font>, <a>, <center>.
		c.warn("已去掉 <%s> 标签，保留其中的内容", tag)
		c.children(ch)
		for k := ch.FirstChild; k != nil; {
			next := k.NextSibling
			ch.RemoveChild(k)
			parent.InsertBefore(k, ch)
			k = next
		}
		parent.RemoveChild(ch)
	}
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func (c *htmlCleaner) style(parent, n *html.Node) {
	if !c.allowStyle {
		c.warn("已删除 <style>")
		parent.RemoveChild(n)
		return
	}
	var text strings.Builder
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		if k.Type == html.TextNode {
			text.WriteString(k.Data)
		}
	}
	clean, warns, err := SanitizeCSS(text.String())
	for _, m := range warns {
		c.w.add("%s", m)
	}
	if err != nil {
		c.warn("已删除一段样式：%s", err.Error())
		parent.RemoveChild(n)
		return
	}
	n.Attr = nil
	for k := n.FirstChild; k != nil; {
		next := k.NextSibling
		n.RemoveChild(k)
		k = next
	}
	n.AppendChild(&html.Node{Type: html.TextNode, Data: clean})
}

func (c *htmlCleaner) meta(parent, n *html.Node) {
	var keep []html.Attribute
	var equiv, content string
	for _, a := range n.Attr {
		switch a.Key {
		case "charset":
			keep = append(keep, html.Attribute{Key: "charset", Val: "UTF-8"})
		case "http-equiv":
			equiv = strings.ToLower(a.Val)
		case "content":
			content = a.Val
		}
	}
	if equiv == "content-security-policy" && content == CSP {
		keep = append(keep, html.Attribute{Key: "http-equiv", Val: "Content-Security-Policy"}, html.Attribute{Key: "content", Val: CSP})
	}
	if len(keep) == 0 {
		c.warn("已删除 <meta>")
		parent.RemoveChild(n)
		return
	}
	n.Attr = keep
}

// attrs keeps the attributes a tag may have.
func (c *htmlCleaner) attrs(n *html.Node, tag string) {
	var keep []html.Attribute
	for _, a := range n.Attr {
		key := strings.ToLower(a.Key)
		if a.Namespace != "" {
			c.warn("已删除属性 %s", key)
			continue
		}
		ok := false
		switch {
		case key == "class" || key == "id" || key == "lang":
			ok = true
		case dataAttr.MatchString(key):
			ok = true
		case key == "style":
			clean, warns := SanitizeDeclarations(a.Val)
			for _, m := range warns {
				c.w.add("%s", m)
			}
			if clean == "" {
				continue
			}
			keep = append(keep, html.Attribute{Key: key, Val: clean})
			continue
		case (tag == "td" || tag == "th") && (key == "colspan" || key == "rowspan"):
			ok = countAttr.MatchString(a.Val)
		case tag == "th" && key == "scope":
			ok = a.Val == "col" || a.Val == "row" || a.Val == "colgroup" || a.Val == "rowgroup"
		case (tag == "col" || tag == "colgroup") && key == "span":
			ok = countAttr.MatchString(a.Val)
		case tag == "ol" && key == "start":
			ok = countAttr.MatchString(a.Val)
		case tag == "img" && key == "src":
			if !AllowedImageURL(a.Val) {
				c.warn("已删除图片地址 %s：只能用内嵌的 png / jpeg / webp 或 asset: 图片", clip(a.Val))
				continue
			}
			ok = true
		case tag == "img" && key == "alt":
			ok = true
		case tag == "img" && (key == "width" || key == "height"):
			ok = lengthAttr.MatchString(a.Val)
		}
		if !ok {
			c.warn("已删除属性 %s", key)
			continue
		}
		keep = append(keep, html.Attribute{Key: key, Val: a.Val})
	}
	n.Attr = keep
}

// PlainText is the text of a piece of HTML, white space collapsed: what a
// slot says when it is drawn as a running head or foot, which are plain text.
func PlainText(frag string) string {
	ctx := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(frag), ctx)
	if err != nil {
		return ""
	}
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			if droppedElements[strings.ToLower(n.Data)] || n.Data == "style" {
				return
			}
			if n.Data == "br" {
				b.WriteString(" ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
