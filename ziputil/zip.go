package ziputil

import (
	"archive/zip"
	"fmt"
	"io"
	"strings"
)

// WriteFile writes a single file inside a zip archive.
func WriteFile(z *zip.Writer, filename string, src io.WriterTo) error {
	writer, err := z.Create(filename)
	if err != nil {
		return fmt.Errorf("ziputil.WriteFile: unable to create %q: %w", filename, err)
	}
	if _, err := src.WriteTo(writer); err != nil {
		return fmt.Errorf("ziputil.WriteFile: unable to write %q: %w", filename, err)
	}
	return nil
}

// StringWriterTo wraps string into io.WriterTo.
type StringWriterTo string

// WriteTo implements io.WriterTo.
func (s StringWriterTo) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, strings.NewReader(string(s)))
}

// WriterToWrapper helps wrapping lambdas into io.WriterTo.
type WriterToWrapper func(w io.Writer) (int64, error)

// WriteTo implements io.WriterTo.
func (w WriterToWrapper) WriteTo(writer io.Writer) (int64, error) {
	return w(writer)
}
