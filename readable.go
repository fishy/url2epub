package url2epub

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/fishy/url2epub/internal/set"
)

// A map of:
// key: atoms we want to keep in the readable html.
// value: the attributes we want to keep inside this atom.
var atoms = map[atom.Atom]set.String{
	atom.Html: set.StringLiteral(
		"lang",
	),
	atom.A: set.StringLiteral(
		"href",
	),
	atom.Img: set.StringLiteral(
		"alt",
		"src",
		"width",
		"height",
	),
	atom.Time: set.StringLiteral(
		"datetime",
	),
	atom.Head:    nil,
	atom.Title:   nil,
	atom.Body:    nil,
	atom.Article: nil,
	atom.Div:     nil,
	atom.H1:      nil,
	atom.H2:      nil,
	atom.H3:      nil,
	atom.H4:      nil,
	atom.H5:      nil,
	atom.H6:      nil,
	atom.P:       nil,
	atom.Ul:      nil,
	atom.Ol:      nil,
	atom.Li:      nil,
	atom.Span:    nil,
	atom.Em:      nil,
	atom.Br:      nil,
}

// Replace some amp elements that's not defined in atoms with their
// atom-equivalents.
var ampAtoms = map[string]atom.Atom{
	"amp-img": atom.Img,
}

// Readable strips node n into a readable one.
func (n *Node) Readable() *html.Node {
	if n == nil {
		return nil
	}
	node := n.AsNode()
	switch node.Type {
	default:
		return nil

	case html.TextNode:
		if strings.TrimSpace(node.Data) == "" {
			// This text node is all white space, skipping.
			return nil
		}
		return &html.Node{
			Type: node.Type,
			Data: node.Data,
		}

	case html.ElementNode:
		// Copy key fields.
		newNode := &html.Node{
			Type:     node.Type,
			DataAtom: node.DataAtom,
			Data:     node.Data,
		}
		if newNode.DataAtom == 0 {
			newNode.DataAtom = ampAtoms[newNode.Data]
			if newNode.DataAtom != 0 {
				newNode.Data = newNode.DataAtom.String()
			}
		}
		attrs, ok := atoms[newNode.DataAtom]
		if !ok {
			// Not an atom we want to keep
			return nil
		}
		for _, attr := range node.Attr {
			if !attrs.Has(attr.Key) {
				continue
			}
			newNode.Attr = append(newNode.Attr, attr)
		}
		n.ForEachChild(func(c *Node) bool {
			child := c.Readable()
			if child == nil {
				return true
			}
			newNode.AppendChild(child)
			return true
		})
		return newNode
	}
}
