package archive

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Package file gzip.go contains the Gzip compression methods.

// Gzip returns the uncompressed filename of the gzip archive.
func (c *Content) Gzip(ctx context.Context, src string) error {
	const format = "content gzip %s %w"

	if err := ctx.Err(); err != nil {
		return fmt.Errorf(format, "timeout", err)
	}

	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf(format, "open src", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf(format, "header", ErrRead)
	}
	defer gzr.Close()

	name := gzr.Name
	if name == "" {
		name = GzipName(src)
	}

	c.Files = append(c.Files, name)
	c.Ext = gzipx
	return nil
}

// GzipTar returns the content of a TAR archive compressed with gzip.
// This process is slower as the gzip file must first be decompressed
// to a temporary directory before the TAR archive can be accessed.
func (c *Content) GzipTar(ctx context.Context, src string) error {
	const format = "content gzip tar %s %w"

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf(format, "open src", err)
	}
	defer f.Close()

	rd, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf(format, "header", ErrRead)
	}
	defer rd.Close()

	name := rd.Name
	if name == "" {
		name = GzipName(src)
	}

	const pattern = "archive-cgz-decompress-*"
	tempDir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return fmt.Errorf(format, "temp directory", err)
	}
	defer os.RemoveAll(tempDir)

	r := bufio.NewReader(rd)
	x := Extractor{
		Source:      "", // not needed
		Destination: tempDir,
	}
	err = x.singleFile(r, name)
	if err != nil {
		return fmt.Errorf(format, "temp directory", err)
	}

	tempTar := filepath.Join(tempDir, name)
	return c.Tar(ctx, tempTar)
}

// Gzip extracts the compressed file from a gzip archive.
// If the compressed file is a supported TAR archive,
// that too is extracted. These combined files often use
// the ".tgz" and ".tar.gz" filename extensions.
//
// The targets are only for TAR archives and are otherwise ignored.
func (x Extractor) Gzip(ctx context.Context, targets ...string) error {
	const format = "extract gzip %s %w"

	if x.Destination == "" {
		return ErrDest
	}

	f, err := os.Open(x.Source)
	if err != nil {
		return fmt.Errorf(format, "open source", err)
	}
	defer f.Close()

	src, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf(format, "header", err)
	}
	defer src.Close()

	r := bufio.NewReader(src)

	if IsTar(r) {
		return x.tarReader(ctx, nil, r, targets...)
	}

	return x.singleFile(r, src.Name)
}

// singleFile extracts the named file from the reader.
func (x Extractor) singleFile(r io.Reader, name string) error {
	const format = "extractor gzip file %s %w"
	s := name
	if s == "" {
		s = GzipName(x.Source)
	}

	path := filepath.Join(x.Destination, filepath.Base(s))
	const perm = WriteWriteRead
	dst, err := os.OpenFile(path, WriteOnly, perm)
	if err != nil {
		return fmt.Errorf(format, "create file", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, r); err != nil {
		return fmt.Errorf(format, "copy", err)
	}
	return nil
}

// GzipName returns the uncompressed base filename of the gzip archive.
//
// For example, if the base filename is `example.txt.gz`, the uncompressed filename is `example.txt`.
func GzipName(src string) string {
	base := filepath.Base(src)
	if i := strings.LastIndex(base, "."); i > 0 {
		return base[:i]
	}
	return base
}
