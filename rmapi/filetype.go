package rmapi

import (
	"bytes"
	"text/template"
)

// FileType is an enum type defining the file type on reMarkable.
//
// It's either epub or pdf.
type FileType int

// FileType values.
const (
	_ FileType = iota
	FileTypeEpub
	FileTypePdf
)

// Ext returns the file extension of the given FileType.
func (ft FileType) Ext() string {
	switch ft {
	default:
		return ""
	case FileTypeEpub:
		return ".epub"
	case FileTypePdf:
		return ".pdf"
	}
}

var (
	tmplEpub = template.Must(template.New("content").Parse(`{
 "fileType": "epub",
 "fontName": "{{.Font}}",
 "lineHeight": 100,
 "margins": 150,
 "orientation": "portrait",
 "textAlignment": "left",
 "textScale": 1,
 "transform": {}
}
`))

	tmplPdf = template.Must(template.New("content").Parse(`{
 "fileType": "pdf",
 "fontName": "{{.Font}}",
 "margins": 100,
 "orientation": "portrait",
 "textAlignment": "left",
 "textScale": 1,
 "transform": {}
}
`))
)

// ContentArgs defines the args to population InitialContent.
type ContentArgs struct {
	Font string
}

// InitialContent returns the initial .content file for the given FileType.
func (ft FileType) InitialContent(args ContentArgs) (string, error) {
	var tmpl *template.Template
	switch ft {
	case FileTypeEpub:
		tmpl = tmplEpub
	case FileTypePdf:
		tmpl = tmplPdf
	}
	if tmpl == nil {
		return "", nil
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return "", err
	}
	return buf.String(), nil
}
