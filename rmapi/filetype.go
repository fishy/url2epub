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

var contentTmpl = template.Must(template.New("content").Parse(`
{
 "fileType": "{{.Type}}",
 "transform": {}
}
`))

// InitialContent returns the initial .content file for the given FileType.
func (ft FileType) InitialContent() (string, error) {
	var data struct {
		Type string
	}
	switch ft {
	case FileTypeEpub:
		data.Type = "epub"
	case FileTypePdf:
		data.Type = "pdf"
	}
	var buf bytes.Buffer
	if err := contentTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
