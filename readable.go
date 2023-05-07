package url2epub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.yhsif.com/immutable"
	"golang.org/x/exp/slog"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"go.yhsif.com/url2epub/grayscale"
)

const (
	imgSrc    = "src"
	imgSrcset = "srcset"
	jpgExt    = ".jpg"
)

var emptyStringSet = immutable.EmptySet[string]()

var imgAtoms = immutable.SetLiteral(atom.Img, atom.Source)

// A map of:
// key: atoms we want to keep in the readable html.
// value: the attributes we want to keep inside this atom.
var atoms = map[atom.Atom]immutable.Set[string]{
	atom.A: immutable.SetLiteral(
		"href",
	),
	atom.Abbr: immutable.SetLiteral(
		"title",
	),
	atom.Acronym: immutable.SetLiteral(
		"title",
	),
	atom.Html: immutable.SetLiteral(
		"lang",
	),
	atom.Img: immutable.SetLiteral(
		imgSrc,
		imgSrcset,
		"alt",
		"width",
		"height",
	),
	atom.Source: immutable.SetLiteral(
		imgSrc,
		imgSrcset,
		"width",
		"height",
		"type",
	),

	atom.Article:    emptyStringSet,
	atom.B:          emptyStringSet,
	atom.Big:        emptyStringSet,
	atom.Blockquote: emptyStringSet,
	atom.Body:       emptyStringSet,
	atom.Br:         emptyStringSet,
	atom.Center:     emptyStringSet,
	atom.Cite:       emptyStringSet,
	atom.Code:       emptyStringSet,
	atom.Content:    emptyStringSet,
	atom.Del:        emptyStringSet,
	atom.Details:    emptyStringSet,
	atom.Dd:         emptyStringSet,
	atom.Dfn:        emptyStringSet,
	atom.Div:        emptyStringSet,
	atom.Dl:         emptyStringSet,
	atom.Dt:         emptyStringSet,
	atom.Em:         emptyStringSet,
	atom.Figure:     emptyStringSet,
	atom.Figcaption: emptyStringSet,
	atom.Footer:     emptyStringSet,
	atom.H1:         emptyStringSet,
	atom.H2:         emptyStringSet,
	atom.H3:         emptyStringSet,
	atom.H4:         emptyStringSet,
	atom.H5:         emptyStringSet,
	atom.H6:         emptyStringSet,
	atom.Head:       emptyStringSet,
	atom.Header:     emptyStringSet,
	atom.I:          emptyStringSet,
	atom.Li:         emptyStringSet,
	atom.Main:       emptyStringSet,
	atom.Mark:       emptyStringSet,
	atom.Noscript:   emptyStringSet,
	atom.Ol:         emptyStringSet,
	atom.P:          emptyStringSet,
	atom.Picture:    emptyStringSet,
	atom.Pre:        emptyStringSet,
	atom.Q:          emptyStringSet,
	atom.S:          emptyStringSet,
	atom.Section:    emptyStringSet,
	atom.Small:      emptyStringSet,
	atom.Span:       emptyStringSet,
	atom.Strike:     emptyStringSet,
	atom.Strong:     emptyStringSet,
	atom.Sub:        emptyStringSet,
	atom.Summary:    emptyStringSet,
	atom.Sup:        emptyStringSet,
	atom.Table:      emptyStringSet,
	atom.Tbody:      emptyStringSet,
	atom.Tfoot:      emptyStringSet,
	atom.Td:         emptyStringSet,
	atom.Th:         emptyStringSet,
	atom.Thead:      emptyStringSet,
	atom.Tr:         emptyStringSet,
	atom.Time:       emptyStringSet,
	atom.Title:      emptyStringSet,
	atom.U:          emptyStringSet,
	atom.Ul:         emptyStringSet,
}

var imgSrcAlternatives = immutable.SetLiteral(
	"nitro-lazy-src",
	"data-src",
)

// The atoms that we need to keep even if they have no attributes and no
// children after stripping.
var keepEmptyAtoms = immutable.SetLiteral(
	atom.Br,
	atom.Td,
)

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

	// If Grayscale is set to true,
	// all images will be grayscaled and encoded as jpegs.
	Grayscale bool
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
		args.Grayscale,
	)
	if err != nil {
		return nil, nil, err
	}
	if head == nil {
		head = &html.Node{
			Type:     html.ElementNode,
			DataAtom: atom.Head,
			Data:     atom.Head.String(),
		}
	}
	head.AppendChild(&html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Meta,
		Data:     atom.Meta.String(),
		Attr: []html.Attribute{
			{
				Key: "itemprop",
				Val: "generated-by: https://pkg.go.dev/go.yhsif.com/url2epub#Node.Readable",
			},
		},
	})
	head.AppendChild(&html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Meta,
		Data:     atom.Meta.String(),
		Attr: []html.Attribute{
			{
				Key: "itemprop",
				Val: "generated-at: " + time.Now().Format(time.RFC3339),
			},
		},
	})
	if args.BaseURL != nil {
		head.AppendChild(&html.Node{
			Type:     html.ElementNode,
			DataAtom: atom.Meta,
			Data:     atom.Meta.String(),
			Attr: []html.Attribute{
				{
					Key: "itemprop",
					Val: "generated-from: " + args.BaseURL.String(),
				},
			},
		})
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
		args.Grayscale,
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
			args.Grayscale,
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

var allowedSrcSchemes = immutable.SetLiteral(
	"", // important for relative image urls
	"https",
	"http",
)

func tryParseImgURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err == nil && allowedSrcSchemes.Contains(u.Scheme) {
		return u
	}
	return nil
}

// Examples:
// * "url 640w"
// * " url 640w"
// * "url"
var srcsetRE = regexp.MustCompile(`^\s*(.+?)(?: (\d+)w)?\s*$`)

func tryParseImgSrcset(s string) *url.URL {
	urls := strings.Split(s, ",")
	var maxWidth int64 = -1
	var maxURL *url.URL
	for _, item := range urls {
		groups := srcsetRE.FindStringSubmatch(item)
		if len(groups) == 0 {
			continue
		}
		// Should not fail based on the regexp
		width, _ := strconv.ParseInt(groups[2], 10, 64)
		if width > maxWidth {
			u := tryParseImgURL(groups[1])
			if u == nil {
				continue
			}
			maxWidth = width
			maxURL = u
		}
	}
	return maxURL
}

func findSrcURLFromIMGNode(
	node *html.Node,
	srcIndices []int,
	srcsetIndex int,
) *url.URL {
	for _, i := range srcIndices {
		if i < 0 {
			continue
		}
		if u := tryParseImgURL(node.Attr[i].Val); u != nil {
			return u
		}
	}
	if srcsetIndex < 0 {
		return nil
	}
	return tryParseImgSrcset(node.Attr[srcsetIndex].Val)
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
	gray bool,
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
		if node.DataAtom == atom.Noscript {
			child := node.FirstChild
			if child == nil || child != node.LastChild || child.Type != html.TextNode {
				// We only care about a single TextNode inside noscript.
				return nil, nil
			}
			childNode, err := html.Parse(strings.NewReader(child.Data))
			if err != nil {
				slog.DebugCtx(
					ctx,
					"Failed to parse noscript data",
					"err", err,
					"data", child.Data,
				)
			}
			if img := FromNode(childNode).FindFirstAtomNode(atom.Img); img != nil {
				node = img.AsNode()
			} else {
				// No img node found
				return nil, nil
			}
		}
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
		srcsetIndex := -1
		var altSrcIndices []int
		for _, attr := range node.Attr {
			i := len(newNode.Attr)
			if newNode.DataAtom == atom.Img && imgSrcAlternatives.Contains(attr.Key) {
				altSrcIndices = append(altSrcIndices, i)
			} else if !attrs.Contains(attr.Key) {
				continue
			}
			newNode.Attr = append(newNode.Attr, attr)
			switch attr.Key {
			case imgSrc:
				srcIndex = i
			case imgSrcset:
				srcsetIndex = i
			}
		}
		if imgAtoms.Contains(newNode.DataAtom) {
			// Special handling for images.
			newNode.DataAtom = atom.Img
			newNode.Data = atom.Img.String()
			srcURL := findSrcURLFromIMGNode(newNode, append([]int{srcIndex}, altSrcIndices...), srcsetIndex)
			if srcURL == nil {
				// No usable src, skip this image
				return nil, nil
			}
			srcURL = baseURL.ResolveReference(srcURL)
			src := srcURL.String()
			if srcIndex < 0 {
				srcIndex = len(newNode.Attr)
				newNode.Attr = append(newNode.Attr, html.Attribute{
					Key: imgSrc,
				})
			}
			if filename, exists := imgMapping[src]; exists {
				// This image url already appeared before, reuse the same local file.
				newNode.Attr[srcIndex].Val = filename
			} else {
				*imgCounter++
				ext := path.Ext(srcURL.Path)
				if gray {
					ext = jpgExt
				}
				filename = fmt.Sprintf("%03d", *imgCounter) + ext
				filename = path.Join(imagesDir, filename)
				newNode.Attr[srcIndex].Val = filename
				imgMapping[src] = filename
				reader := new(io.Reader)
				images[filename] = reader
				wg.Add(1)
				go func() {
					defer wg.Done()
					downloadImage(ctx, srcURL, userAgent, reader, gray)
				}()
			}
			// Remove srcset if they are there
			if srcsetIndex >= 0 {
				newNode.Attr = append(
					newNode.Attr[:srcsetIndex],
					newNode.Attr[srcsetIndex+1:]...,
				)
			}
			// Skip adding childrens to img tags.
			// When img tags have childrens, the render will fail with:
			//     html: void element <img> has child nodes
			// But amp-img tags are actually allowed to have children (fallbacks)
			// For those cases, just drop them.
			// See https://github.com/fishy/url2epub/issues/3.
			return newNode, nil
		}
		var iterationErr error
		n.ForEachChild(func(c *Node) bool {
			child, err := c.readableRecursive(ctx, wg, baseURL, userAgent, imagesDir, images, imgMapping, imgCounter, gray)
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
		if len(newNode.Attr) == 0 && newNode.FirstChild == nil && !keepEmptyAtoms.Contains(newNode.DataAtom) {
			// This node has no children and no attributes, skipping
			return nil, nil
		}
		return newNode, nil
	}
}

func downloadImage(ctx context.Context, src *url.URL, userAgent string, dest *io.Reader, gray bool) {
	body, _, err := get(ctx, src, userAgent)
	if err != nil {
		return
	}
	if !gray {
		*dest = body
		return
	}
	defer DrainAndClose(body)
	img, err := grayscale.FromReader(body)
	if err != nil {
		slog.ErrorCtx(
			ctx,
			"Error while trying to grayscale",
			"err", err,
			"url", src.String(),
		)
		return
	}
	reader, err := img.ToJPEG()
	if err != nil {
		slog.ErrorCtx(
			ctx,
			"Error while trying to encode grayscaled %q: %v",
			"err", err,
			"url", src.String(),
		)
		return
	}
	*dest = reader
}
