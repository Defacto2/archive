package archive

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Defacto2/archive/sanitize"
)

// TODO: create more tar tests
// bsdtar -c --format "ustar" "pax" etc

// Package file tar.go contains the Tape ARchives methods.

// Tar returns the content of a TAR archive.
func (c *Content) Tar(ctx context.Context, src string) error {
	const format = "content tar %s %w"

	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf(format, "open src", err)
	}
	defer file.Close()

	hdr := tar.NewReader(file)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(format, "timeout", ctx.Err())
		default:
		}

		header, err := hdr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf(format, "read entry", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			c.Files = append(c.Files, header.Name)
		}
	}
	if len(c.Files) > 0 {
		c.Ext = tarx
	}
	return nil
}

// Tar extracts the content of the Tar archive.
// If the targets are empty then all files are extracted.
// Any errors with the archive or its content are ignored.
func (x Extractor) Tar(ctx context.Context, targets ...string) error {
	logger := slog.New(slog.DiscardHandler)
	return x.TarWithLogger(ctx, logger, targets...)
}

// TarWithLogger extracts the content of the Tar archive and reports any
// archive or extraction errors to the logger.
// If the targets are empty then all files are extracted.
func (x Extractor) TarWithLogger(ctx context.Context, logger *slog.Logger, targets ...string) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	const msg = "extract tar"
	const format = msg + " %s %w"
	if x.Destination == "" {
		logger.Error(msg + " extractor.destination is empty")
		return ErrDest
	}

	f, err := os.Open(x.Source)
	if err != nil {
		logger.Error(msg+" cannot open extractor.source",
			slog.String("source", x.Source), slog.Any("error", err))
		return fmt.Errorf(format, "open source", err)
	}
	defer f.Close()

	return x.tarReader(ctx, logger, f, targets...)
}

func (x Extractor) tarReader(ctx context.Context, logger *slog.Logger, r io.Reader, targets ...string) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	const msg = "extract tar"
	const format = msg + " %s %w"
	logErr := func(s string, err error) {
		logger.Error(msg+""+s, slog.Any("error", err))
	}

	src := tar.NewReader(r)
	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			logErr("reader timed out", err)
			return fmt.Errorf(format, "timeout", err)
		default:
		}

		hdr, err := src.Next()
		if errors.Is(err, io.EOF) {
			logger.Debug(msg + " reached the end of the archive")
			break
		}
		if err != nil {
			logErr("cannot read header", err)
			return fmt.Errorf(format, "reader header", err)
		}
		if skipName(hdr.Name, targets...) {
			logger.Info(msg+" skipping entry", slog.String("name", hdr.Name))
			continue
		}

		local := sanitize.Name(hdr.Name)
		if local != hdr.Name {
			logger.Info(msg+" renamed entry",
				slog.String("stored name", hdr.Name), slog.String("extracted name", local))
		}
		path := filepath.Join(x.Destination, local)
		x.tarEntry(logger, src, hdr, path)
	}

	return nil
}

func (x Extractor) tarEntry(
	logger *slog.Logger, src *tar.Reader, hdr *tar.Header, path string,
) {
	if logger == nil || src == nil || hdr == nil {
		return
	}

	const msg = "extract tar entry cannot "
	logPaths := slog.Group("paths",
		slog.String("header name", hdr.Name), slog.String("path", path),
	)
	logErr := func(s string, err error) {
		logger.Error(msg+s, logPaths, slog.Any("error", err))
	}

	switch hdr.Typeflag {
	case tar.TypeLink, tar.TypeSymlink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
		logger.Debug(msg+"skipping node or link entry", logPaths)
	case tar.TypeDir:
		const perm = DirWriteReadRead
		if err := os.MkdirAll(path, perm); err != nil {
			logErr("make a directory", err)
			return
		}
	case tar.TypeReg:
		perm := DirWriteReadRead
		parent := filepath.Dir(path)
		if err := os.MkdirAll(parent, perm); err != nil {
			logErr("make parent directory", err)
			return
		}
		const flag = WriteRead
		perm = hdr.FileInfo().Mode()
		dst, err := os.OpenFile(path, flag, perm)
		if err != nil {
			logErr("open file", err)
			return
		}

		if hdr.Size < 0 {
			logErr("opened size", ErrSize)
			return
		}
		n, err := io.CopyN(dst, src, hdr.Size)
		cErr := dst.Close()
		if err != nil {
			logErr("copy file", err)
			return
		}
		if cErr != nil {
			logErr("flush destination file", cErr)
			return
		}
		logger.Debug(msg+" extracted file",
			slog.String("created path", path), slog.Int64("bytes written", n))

		if err := tarTimes(logger, hdr, path); err != nil {
			logErr("set access times", err)
		}
	}
}

func tarTimes(logger *slog.Logger, hdr *tar.Header, path string) error {
	if logger == nil || hdr == nil {
		return nil
	}
	atime := hdr.AccessTime
	mtime := hdr.ModTime
	if mtime.IsZero() {
		mtime = time.Now()
	}
	if atime.IsZero() {
		atime = mtime
	}
	if err := os.Chtimes(path, atime, mtime); err != nil {
		return fmt.Errorf("tar times %w", err)
	}
	return nil
}

// IsTar returns true if the reader is a supported tar format.
func IsTar(r *bufio.Reader) bool {
	if r == nil {
		return false
	}
	const size = 512
	peek, err := r.Peek(size)
	if err != nil || len(peek) < size {
		return false
	}

	const prefix = "ustar"
	magic := string(peek[257:263])
	if strings.HasPrefix(magic, prefix) {
		return true
	}

	// legacy tar archives
	// pass a copy of the peeked header bytes into standard tar.Reader
	tr := tar.NewReader(bytes.NewReader(peek))
	hdr, err := tr.Next()
	if err != nil || hdr == nil {
		return false
	}
	return hdr.Name != "" && (hdr.Mode > 0 || hdr.Size >= 0)
}
