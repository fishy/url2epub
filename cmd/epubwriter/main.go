package main

import (
	"archive/zip"
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"go.yhsif.com/url2epub/ziputil"
	"golang.org/x/exp/slog"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, nil))

const (
	mimetypeFileName = "mimetype"
)

var (
	from = flag.String(
		"from",
		"",
		"The directory containing the files",
	)
)

func main() {
	flag.Parse()
	if code := run(); code != 0 {
		os.Exit(code)
	}
}

func run() (code int) {
	z := zip.NewWriter(os.Stdout)
	defer func() {
		if err := z.Close(); err != nil {
			logger.Error("Failed to close zip archive", "err", err)
			code = -1
		}
	}()

	dir := os.DirFS(*from)
	mimetype, err := dir.Open(mimetypeFileName)
	if err != nil {
		logger.Error(
			"Failed to read mimetype file",
			"err", err,
			"file", mimetypeFileName,
		)
		return -1
	}

	// mimetype must be the first file in the zip,
	// and must use Store instead of Deflate.
	if err := ziputil.StoreFile(z, mimetypeFileName, readCloserWriterTo(mimetype)); err != nil {
		logger.Error(
			"Failed to write mimetype file",
			"err", err,
			"file", mimetypeFileName,
		)
		return -1
	}

	const root = "."
	if err := fs.WalkDir(dir, root, func(path string, d fs.DirEntry, e error) (err error) {
		defer func() {
			if err != nil {
				logger.Error(
					"Failed to handle file",
					"err", err,
					"file", path,
				)
			}
		}()

		if e != nil {
			return e
		}

		if d.IsDir() {
			// skip
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			// Should not happen but just in case
			return err
		}
		if relPath == mimetypeFileName {
			// skip
			return nil
		}

		f, err := dir.Open(relPath)
		if err != nil {
			return err
		}
		return ziputil.WriteFile(z, relPath, readCloserWriterTo(f))
	}); err != nil {
		return -1
	}

	return 0
}

func readCloserWriterTo(r io.ReadCloser) io.WriterTo {
	return ziputil.WriterToWrapper(func(w io.Writer) (int64, error) {
		defer func() {
			r.Close()
		}()
		return io.Copy(w, r)
	})
}
