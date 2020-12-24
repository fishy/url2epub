package url2epub

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/fishy/url2epub/internal/set"
)

const imgSrc = "src"

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
		imgSrc,
		"alt",
		"width",
		"height",
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
	atom.Time:    nil,
	atom.Span:    nil,
	atom.Em:      nil,
	atom.Br:      nil,
}

// Replace some amp elements that's not defined in atoms with their
// atom-equivalents.
var ampAtoms = map[string]atom.Atom{
	"amp-img": atom.Img,
}

// Readable strips node n into a readable one, with all images downloaded and
// replaced.
func (n *Node) Readable(
	ctx context.Context,
	baseURL *url.URL,
	userAgent string,
	imagesDir string,
) (*html.Node, map[string]io.Reader, error) {
	images := make(map[string]io.Reader)
	var wg sync.WaitGroup
	var counter int
	node, err := n.readableRecursive(ctx, &wg, baseURL, userAgent, imagesDir, images, &counter)
	wg.Wait()
	return node, images, err
}

func (n *Node) readableRecursive(
	ctx context.Context,
	wg *sync.WaitGroup,
	baseURL *url.URL,
	userAgent string,
	imagesDir string,
	images map[string]io.Reader,
	imgCounter *int,
) (*html.Node, error) {
	if n == nil {
		return nil, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	node := n.AsNode()
	switch node.Type {
	default:
		return nil, nil

	case html.TextNode:
		if strings.TrimSpace(node.Data) == "" {
			// This text node is all white space, skipping.
			return nil, nil
		}
		return &html.Node{
			Type: node.Type,
			Data: node.Data,
		}, nil

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
			// Not an atom we want to keep.
			return nil, nil
		}
		srcIndex := -1
		for _, attr := range node.Attr {
			if !attrs.Has(attr.Key) {
				continue
			}
			i := len(newNode.Attr)
			newNode.Attr = append(newNode.Attr, attr)
			if attr.Key == imgSrc {
				srcIndex = i
			}
		}
		if newNode.DataAtom == atom.Img {
			// Special handling for images.
			if srcIndex < 0 {
				// No src, skip this image
				return nil, nil
			}
			srcURL, err := url.Parse(newNode.Attr[srcIndex].Val)
			if err != nil {
				// Unparsable image source url, skip this image
				return nil, nil
			}
			srcURL = baseURL.ResolveReference(srcURL)
			*imgCounter++
			filename := fmt.Sprintf("%03d", *imgCounter) + path.Ext(srcURL.Path)
			filename = path.Join(imagesDir, filename)
			newNode.Attr[srcIndex].Val = filename
			buf := new(bytes.Buffer)
			images[filename] = buf
			wg.Add(1)
			go func() {
				defer wg.Done()
				downloadImage(ctx, srcURL, userAgent, buf)
			}()
		}
		var iterationErr error
		n.ForEachChild(func(c *Node) bool {
			child, err := c.readableRecursive(ctx, wg, baseURL, userAgent, imagesDir, images, imgCounter)
			if err != nil {
				iterationErr = err
				return false
			}
			if child == nil {
				return true
			}
			newNode.AppendChild(child)
			return true
		})
		if iterationErr != nil {
			return nil, iterationErr
		}
		if len(newNode.Attr) == 0 && newNode.FirstChild == nil {
			// This node has no children and no attributes, skipping
			return nil, nil
		}
		return newNode, nil
	}
}

func downloadImage(ctx context.Context, src *url.URL, userAgent string, dest io.Writer) {
	req := setContextUserAgent(
		ctx,
		&http.Request{
			Method: http.MethodGet,
			URL:    src,
			Header: make(http.Header),
		},
		userAgent,
	)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer DrainAndClose(resp.Body)
	io.Copy(dest, resp.Body)
}
