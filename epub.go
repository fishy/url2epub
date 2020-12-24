package url2epub

import (
	"archive/zip"
	"bufio"
	"bytes"
	"io"
	"net/http"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/html"
)

const (
	contentTypePeekSize = 512

	epubMimetypeFilename = `mimetype`
	epubMimetypeContent  = `application/epub+zip`

	epubContainerFilename = `META-INF/container.xml`
	epubContainerContent  = `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
 <rootfiles>
  <rootfile full-path="` + epubOpfFullpath + `" media-type="application/oebps-package+xml"/>
 </rootfiles>
</container>
`

	epubContentDir      = "content"
	epubArticleFilename = "article.xhtml"

	epubOpfFullpath = epubContentDir + "/content.opf"
	epubOpfTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="BookID">
 <metadata xmlns:dc="http://purl.org/dc/elements/0.1/">
  <dc:identifier id="BookID">{{.ID}}</dc:identifier>
  <dc:title>{{.Title}}</dc:title>
  {{if .Lang -}}
	<dc:language>{{.Lang}}</dc:language>
	{{- end}}
  <meta property="dcterms:modified">{{.Time}}</meta>
 </metadata>
 <manifest>
  <item id="{{.ArticlePath}}" href="{{.ArticlePath}}" media-type="application/xhtml+xml"/>
  {{range $path, $type := .Images}}
  <item id="{{$path}}" href="{{$path}}" media-type="{{$type}}"/>
	{{- end}}
 </manifest>
 <spine>
  <itemref idref="{{.ArticlePath}}"/>
 </spine>
</package>
`
)

var epubOpfTmpl = template.Must(template.New("opf").Parse(epubOpfTemplate))

type epubOpfData struct {
	ID          string
	Title       string
	Lang        string
	Time        string
	ArticlePath string
	Images      map[string]string
}

// EpubArgs defines the args used by Epub function.
type EpubArgs struct {
	// The destination to write the epub content to.
	Dest io.Writer

	// The title of the epub.
	Title string

	// The node pointing to the html tag.
	Node *html.Node

	// Images map:
	// key: image local filename
	// value: image content
	Images map[string]io.Reader
}

// Epub creates an Epub 3.0 file from given content.
func Epub(args EpubArgs) (id string, err error) {
	var randomID uuid.UUID
	randomID, err = uuid.NewRandom()
	if err != nil {
		return
	}
	id = randomID.String()

	z := zip.NewWriter(args.Dest)
	defer func() {
		closeErr := z.Close()
		if err == nil {
			err = closeErr
		}
	}()

	var writer io.Writer

	// mimetype must be the first file in the zip
	writer, err = z.Create(epubMimetypeFilename)
	if err != nil {
		return
	}
	_, err = io.Copy(writer, strings.NewReader(epubMimetypeContent))
	if err != nil {
		return
	}

	writer, err = z.Create(epubContainerFilename)
	if err != nil {
		return
	}
	_, err = io.Copy(writer, strings.NewReader(epubContainerContent))
	if err != nil {
		return
	}

	writer, err = z.Create(path.Join(epubContentDir, epubArticleFilename))
	if err != nil {
		return
	}
	err = html.Render(writer, args.Node)
	if err != nil {
		return
	}

	imageContentTypes := make(map[string]string, len(args.Images))
	for filename, reader := range args.Images {
		err = func() (err error) {
			if readCloser, ok := reader.(io.ReadCloser); ok {
				defer DrainAndClose(readCloser)
			}
			writer, err = z.Create(path.Join(epubContentDir, filename))
			if err != nil {
				return
			}
			var buf []byte
			if buffer, ok := reader.(*bytes.Buffer); ok {
				buf = buffer.Bytes()
			} else {
				r := bufio.NewReader(reader)
				var peekErr error
				buf, peekErr = r.Peek(contentTypePeekSize)
				if peekErr != nil && peekErr != bufio.ErrBufferFull {
					err = peekErr
					return
				}
				reader = r
			}
			imageContentTypes[filename] = http.DetectContentType(buf)
			_, err = io.Copy(writer, reader)
			if err != nil {
				return
			}
			return nil
		}()
		if err != nil {
			return
		}
	}

	writer, err = z.Create(epubOpfFullpath)
	if err != nil {
		return
	}
	err = epubOpfTmpl.Execute(writer, epubOpfData{
		ID:          id,
		Title:       args.Title,
		Lang:        FromNode(args.Node).GetLang(),
		Time:        time.Now().Format(time.RFC3339),
		ArticlePath: epubArticleFilename,
		Images:      imageContentTypes,
	})
	return
}
