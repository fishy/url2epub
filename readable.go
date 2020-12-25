package url2epub

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	atom.A: set.StringLiteral(
		"href",
	),
	atom.Html: set.StringLiteral(
		"lang",
	),
	atom.Img: set.StringLiteral(
		imgSrc,
		"alt",
		"width",
		"height",
	),

	atom.Acronym:    nil,
	atom.Article:    nil,
	atom.B:          nil,
	atom.Big:        nil,
	atom.Blockquote: nil,
	atom.Body:       nil,
	atom.Br:         nil,
	atom.Center:     nil,
	atom.Code:       nil,
	atom.Del:        nil,
	atom.Details:    nil,
	atom.Div:        nil,
	atom.Em:         nil,
	atom.Footer:     nil,
	atom.H1:         nil,
	atom.H2:         nil,
	atom.H3:         nil,
	atom.H4:         nil,
	atom.H5:         nil,
	atom.H6:         nil,
	atom.Head:       nil,
	atom.Header:     nil,
	atom.I:          nil,
	atom.Li:         nil,
	atom.Main:       nil,
	atom.Mark:       nil,
	atom.Ol:         nil,
	atom.P:          nil,
	atom.Pre:        nil,
	atom.Q:          nil,
	atom.S:          nil,
	atom.Section:    nil,
	atom.Small:      nil,
	atom.Span:       nil,
	atom.Strike:     nil,
	atom.Strong:     nil,
	atom.Sub:        nil,
	atom.Summary:    nil,
	atom.Sup:        nil,
	atom.Table:      nil,
	atom.Tbody:      nil,
	atom.Tfoot:      nil,
	atom.Th:         nil,
	atom.Thead:      nil,
	atom.Tr:         nil,
	atom.Time:       nil,
	atom.Title:      nil,
	atom.U:          nil,
	atom.Ul:         nil,
}

// Replace some amp elements that's not defined in atoms with their
// atom-equivalents.
var ampAtoms = map[string]atom.Atom{
	"amp-img": atom.Img,
}

// ReadableArgs defines the args used by Readable function.
type ReadableArgs struct {
	// Base URL of the document, used in case the image URLs are relative.
	BaseURL *url.URL

	// User-Agent to be used to download images.
	UserAgent string

	// Directory prefix for downloaded images.
	ImagesDir string
}

// Readable strips node n into a readable one, with all images downloaded and
// replaced.
func (n *Node) Readable(ctx context.Context, args ReadableArgs) (*html.Node, map[string]io.Reader, error) {
	imgPointers := make(map[string]*io.Reader)
	imgMapping := make(map[string]string)
	var wg sync.WaitGroup
	var counter int

	head, err := n.FindFirstAtomNode(atom.Head).readableRecursive(
		ctx,
		&wg,
		args.BaseURL,
		args.UserAgent,
		args.ImagesDir,
		imgPointers,
		imgMapping,
		&counter,
	)
	if err != nil {
		return nil, nil, err
	}

	var body *html.Node
	article, err := n.FindFirstAtomNode(atom.Article).readableRecursive(
		ctx,
		&wg,
		args.BaseURL,
		args.UserAgent,
		args.ImagesDir,
		imgPointers,
		imgMapping,
		&counter,
	)
	if err != nil {
		return nil, nil, err
	}
	if article == nil {
		body, err = n.FindFirstAtomNode(atom.Body).readableRecursive(
			ctx,
			&wg,
			args.BaseURL,
			args.UserAgent,
			args.ImagesDir,
			imgPointers,
			imgMapping,
			&counter,
		)
		if err != nil {
			return nil, nil, err
		}
		if body == nil {
			return nil, nil, errors.New("no body tag found")
		}
	} else {
		body = &html.Node{
			Type:     html.ElementNode,
			DataAtom: atom.Body,
			Data:     atom.Body.String(),
		}
		body.AppendChild(article)
	}

	root := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Html,
		Data:     atom.Html.String(),
	}
	if head != nil {
		root.AppendChild(head)
	}
	root.AppendChild(body)

	wg.Wait()
	images := make(map[string]io.Reader, len(imgPointers))
	for k, v := range imgPointers {
		var reader io.Reader
		if v != nil && *v != nil {
			reader = *v
		} else {
			reader = strings.NewReader("")
		}
		images[k] = reader
	}
	return root, images, err
}

func (n *Node) readableRecursive(
	ctx context.Context,
	wg *sync.WaitGroup,
	baseURL *url.URL,
	userAgent string,
	imagesDir string,
	images map[string]*io.Reader,
	imgMapping map[string]string,
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
			src := srcURL.String()
			if filename, exists := imgMapping[src]; exists {
				// This image url already appeared before, reuse the same local file.
				newNode.Attr[srcIndex].Val = filename
			} else {
				*imgCounter++
				filename = fmt.Sprintf("%03d", *imgCounter) + path.Ext(srcURL.Path)
				filename = path.Join(imagesDir, filename)
				newNode.Attr[srcIndex].Val = filename
				imgMapping[src] = filename
				reader := new(io.Reader)
				images[filename] = reader
				wg.Add(1)
				go func() {
					defer wg.Done()
					downloadImage(ctx, srcURL, userAgent, reader)
				}()
			}
		}
		var iterationErr error
		n.ForEachChild(func(c *Node) bool {
			child, err := c.readableRecursive(ctx, wg, baseURL, userAgent, imagesDir, images, imgMapping, imgCounter)
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

func downloadImage(ctx context.Context, src *url.URL, userAgent string, dest *io.Reader) {
	body, _, err := get(ctx, src, userAgent)
	if err != nil {
		return
	}
	*dest = body
}
