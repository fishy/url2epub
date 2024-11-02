package url2epub

import (
	"iter"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Node is typedef'd html.Node with helper functions attached.
type Node html.Node

// FromNode casts *html.Node into *Node.
func FromNode(n *html.Node) *Node {
	if n == nil {
		return nil
	}
	ret := Node(*n)
	return &ret
}

// AsNode casts n back to *html.Node
func (n Node) AsNode() html.Node {
	return html.Node(n)
}

// ForEachChild calls f on each of n's children.
//
// If f returns false, ForEachChild stops the iteration.
func (n Node) ForEachChild(f func(child *Node) bool) {
	for c := n.AsNode().FirstChild; c != nil; c = c.NextSibling {
		if !f(FromNode(c)) {
			return
		}
	}
}

// Children returns an iterator for all n's children.
func (n Node) Children() iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		n.ForEachChild(yield)
	}
}

// FindFirstAtomNode returns n itself or the first node in its descendants,
// with Type == html.ElementNode and DataAtom == a, using depth first search.
//
// If none of n's descendants matches, nil will be returned.
func (n *Node) FindFirstAtomNode(a atom.Atom) *Node {
	if n == nil {
		return nil
	}

	if node := n.AsNode(); node.Type == html.ElementNode && node.DataAtom == a {
		return n
	}
	var found *Node
	for c := range n.Children() {
		if ret := c.FindFirstAtomNode(a); ret != nil {
			found = ret
			break
		}
	}
	return found
}

// IsAMP returns true if root is an AMP html document.
func (n *Node) IsAMP() bool {
	n = n.FindFirstAtomNode(atom.Html)
	if n == nil {
		return false
	}
	for _, attr := range n.Attr {
		if attr.Key == "amp" || attr.Key == "⚡" {
			return true
		}
	}
	return false
}

// GetLang returns the lang attribute of html node, if any.
func (n *Node) GetLang() string {
	n = n.FindFirstAtomNode(atom.Html)
	if n == nil {
		return ""
	}
	for _, attr := range n.Attr {
		if attr.Key == langKey {
			return attr.Val
		}
	}
	return ""
}

// GetAMPurl returns the amp URL of the document, if any.
func (n *Node) GetAMPurl() string {
	head := n.FindFirstAtomNode(atom.Head)
	if head == nil {
		return ""
	}
	var found string
	for cc := range head.Children() {
		c := cc.AsNode()
		if c.Type != html.ElementNode || c.DataAtom != atom.Link {
			continue
		}
		m := buildAttrMap(&c)
		if m["rel"] == "amphtml" {
			found = m["href"]
			break
		}
	}
	return found
}

// GetTitle returns the title of the document, if any.
//
// Note that if og:title exists in the meta header, it's preferred over title.
func (n *Node) GetTitle() (title string) {
	defer func() {
		title = html.UnescapeString(title)
	}()

	head := n.FindFirstAtomNode(atom.Head)
	if head == nil {
		return ""
	}

	// Try to find og:title.
	for cc := range head.Children() {
		c := cc.AsNode()
		if c.Type != html.ElementNode || c.DataAtom != atom.Meta {
			continue
		}
		m := buildAttrMap(&c)
		if m["property"] == "og:title" {
			title = m["content"]
			break
		}
	}
	if title != "" {
		return title
	}

	titleNode := head.FindFirstAtomNode(atom.Title)
	if titleNode == nil {
		return ""
	}
	for cc := range titleNode.Children() {
		c := cc.AsNode()
		if c.Type == html.TextNode {
			return strings.TrimSpace(c.Data)
		}
	}
	return ""
}

// GetAuthor returns the author of the document, if any.
func (n *Node) GetAuthor() (author string) {
	defer func() {
		author = html.UnescapeString(author)
	}()

	head := n.FindFirstAtomNode(atom.Head)
	if head == nil {
		return ""
	}

	for cc := range head.Children() {
		c := cc.AsNode()
		if c.Type != html.ElementNode || c.DataAtom != atom.Meta {
			continue
		}
		m := buildAttrMap(&c)
		if m["name"] == "author" {
			return m["content"]
		}
		if m["property"] == "author" {
			return m["content"]
		}
	}
	return ""
}

func buildAttrMap(node *html.Node) map[string]string {
	m := make(map[string]string, len(node.Attr))
	for _, attr := range node.Attr {
		m[attr.Key] = attr.Val
	}
	return m
}
