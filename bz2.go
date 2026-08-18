package archive

import (
	"compress/bzip2"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Defacto2/archive/sanitize"
)

// Package file bz2.go contains the BZ2 compression methods.

// Bz2 returns the uncompressed filename of the bz2 archive.
func (c *Content) Bz2(ctx context.Context, src string) error {
	const format = "context bz2 %s %w"
	if ctx.Err() != nil {
		return fmt.Errorf(format, "timeout", ctx.Err())
	}

	ext := filepath.Ext(src)
	lowerExt := strings.ToLower(ext)
	var path string
	switch lowerExt {
	case ".tbz", ".tbz2":
		path = strings.TrimSuffix(src, ext) + ".tar"
	case "": // fallback if no extension
		path = src + ".out"
	default: // trims .tar.bz2 etc.
		path = strings.TrimSuffix(src, ext)
	}

	name := filepath.Base(path)
	base := sanitize.Name(name)
	c.Files = append(c.Files, base)
	c.Ext = bz2x
	return nil
}

func (x Extractor) Bz2(ctx context.Context) error {
	const format = "extract bz2 %s %w"

	if x.Destination == "" {
		return ErrDest
	}

	if ctx.Err() != nil {
		return fmt.Errorf(format, "timeout", ctx.Err())
	}

	f, err := os.Open(x.Source)
	if err != nil {
		return fmt.Errorf(format, "open source", err)
	}
	defer f.Close()

	name := ""
	var c Content
	if err := c.Bz2(ctx, x.Source); err != nil {
		return fmt.Errorf(format, x.Source, err)
	}
	if len(c.Files) > 0 {
		name = c.Files[0]
	}
	if name == "" {
		name = "extracted.out"
	}

	path := filepath.Join(x.Destination, name)
	const perm = WriteWriteRead

	dst, err := os.OpenFile(path, WriteOnly, perm)
	if err != nil {
		return fmt.Errorf(format, "create file", err)
	}
	defer func() {
		cErr := dst.Close()
		if err != nil {
			_ = os.Remove(path)
			if cErr != nil {
				err = errors.Join(err, fmt.Errorf(format, "closing destination", cErr))
			}
		}
	}()

	const max1GiB = 1 * 1024 * 1024 * 1024
	src := io.LimitReader(bzip2.NewReader(f), max1GiB)
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf(format, "copy", err)
	}

	return nil
}

func (c *Content) Bz2Tar(ctx context.Context, src string) error {
	const format = "content bz2 tar %s %w"
	// 1 GiB limit against decompression bombs (gosec G110)
	const maxExtractSize = 1 * 1024 * 1024 * 1024

	if ctx.Err() != nil {
		return fmt.Errorf(format, "timeout", ctx.Err())
	}

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf(format, "open src", err)
	}
	defer f.Close()

	// Determine output tar filename (e.g., "archive.tar")
	if err := c.Bz2(ctx, src); err != nil {
		return fmt.Errorf(format, src, err)
	}
	name := "archive.tar"
	if len(c.Files) > 0 {
		name = c.Files[0]
	}
	// Clear c.Files so it can be re-populated with the actual tar contents
	c.Files = c.Files[:0]

	const pattern = "archive-bz2-decompress-*"
	tempDir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return fmt.Errorf(format, "temp directory", err)
	}
	defer os.RemoveAll(tempDir)

	tempTarPath := filepath.Join(tempDir, name)
	const perm = WriteWriteRead
	const flags = WriteOnly | os.O_CREATE | os.O_TRUNC

	dst, err := os.OpenFile(tempTarPath, flags, perm)
	if err != nil {
		return fmt.Errorf(format, "create temp tar", err)
	}

	// Safely close the temporary target file before inspecting it
	bzReader := io.LimitReader(bzip2.NewReader(f), maxExtractSize)
	if _, err = io.Copy(dst, bzReader); err != nil {
		_ = dst.Close()
		return fmt.Errorf(format, "decompress bz2", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf(format, "close temp tar", err)
	}

	// Inspect the decompressed .tar file
	if err := c.Tar(ctx, tempTarPath); err != nil {
		return fmt.Errorf(format, "read tar contents", err)
	}

	c.Ext = bz2x
	return nil
}
