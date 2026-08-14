package archive

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Defacto2/archive/command"
)

// Package file zip.go contains the ZIP package and compression methods.

// Zip returns the content of the src ZIP archive.
// The format is credited to Phil Katz.
func (c *Content) Zip(ctx context.Context, src string) error {
	const format = "context zip %s %w"

	if err := ctx.Err(); err != nil {
		return fmt.Errorf(format, "timeout", err)
	}

	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf(format, "open reader", err)
	}
	defer r.Close()

	for _, f := range r.File {
		c.Files = append(c.Files, f.Name)
	}
	c.Ext = zipx
	return nil
}

// ZipInfo returns the content of the src ZIP archive using the [zipinfo program].
// The zip format is credited to Phil Katz.
//
// The use of [Content.Zip] is preferred over this method.
//
// [zipinfo program]: https://infozip.sourceforge.net/
func (c *Content) ZipInfo(ctx context.Context, src string) error {
	const format = "content zipinfo %s %w"
	const file = command.ZipInfo
	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const list = "-1"
	const stopParsing = "--" // prevent files named with "-" from being parsed as flags
	out, err := c.Run(ctx, file, prog, list, stopParsing, src)
	if err != nil {
		return err
	}

	if len(out) == 0 {
		return ErrRead
	}

	c.Files = ZipInfo(out)
	c.Ext = zipx
	return nil
}

// ZipInfo cleans and splits the raw "zipinfo -1" output into a slice of filenames.
// It is needed by [Content.ZipInfo] and otherwise can be ignored.
func ZipInfo(out []byte) []string {
	files := strings.Split(string(out), "\n")
	files = slices.DeleteFunc(files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	for i, f := range files {
		files[i] = strings.TrimRight(f, "\r")
	}
	return files
}

// Zip extracts the content of the src ZIP archive.
// The format is credited to Phil Katz.
// If the targets are empty then all files are extracted.
func (x Extractor) Zip(ctx context.Context, targets ...string) error {
	logger := slog.New(slog.DiscardHandler)
	return x.ZipWithLogger(ctx, logger, targets...)
}

func (x Extractor) ZipWithLogger(ctx context.Context, logger *slog.Logger, targets ...string) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	const msg = "extract zip"
	const format = msg + " %s %w"
	if x.Destination == "" {
		logger.Error(msg + " extractor.destination is empty")
		return ErrDest
	}

	f, err := zip.OpenReader(x.Source)
	if err != nil {
		logger.Error(msg+" cannot open extractor.source",
			slog.String("source", x.Source), slog.Any("error", err))
		return fmt.Errorf(format, "open source", err)
	}
	defer f.Close()

	return x.zipReader(ctx, logger, f, targets...)
}

func (x Extractor) zipReader( //nolint:funlen
	ctx context.Context, logger *slog.Logger, r *zip.ReadCloser, targets ...string,
) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if r == nil {
		return nil
	}

	const msg = "extract zip"
	const format = msg + " %s %w"
	for n, f := range r.File {
		logName := slog.Group("entry", slog.Int("#", n), slog.String("stored name", f.Name))
		logErr := func(s string, err error) {
			logger.Error(msg+" "+s, logName, slog.Any("error", err))
		}

		select {
		case <-ctx.Done():
			err := ctx.Err()
			logErr("reader timeout", err)
			return fmt.Errorf(format, "timeout", err)
		default:
		}

		if skipName(f.Name, targets...) {
			logger.Info(msg+" skipped", logName)
			continue
		}

		local := Localize(f.Name)
		if local != f.Name {
			logger.Info(msg+" renamed", logName, slog.String("extracted name", local))
		}
		path := filepath.Join(x.Destination, local)

		// create directories directly without opening file handles
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, DirWriteReadRead); err != nil {
				logErr("cannot create directory", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), DirWriteReadRead); err != nil {
			logErr("cannot create parent directory", err)
			continue
		}

		rc, err := f.Open()
		if err != nil {
			logErr("cannot open entry", err)
			continue
		}

		const flag = WriteRead
		dst, err := os.OpenFile(path, flag, WriteWriteRead)
		if err != nil {
			logErr("cannot open destination", err)
			rc.Close()
			continue
		}

		n, err := zipCopier(dst, rc, compSize(f))
		if err != nil {
			logErr("copy", err)
			continue
		}
		logger.Debug(msg+" extracted file", logName, slog.Int64("bytes written", n))

		if err := zipTimes(f, path); err != nil {
			logErr("set modified time", err)
		}
	}

	return nil
}

func zipCopier(dst *os.File, rc io.ReadCloser, n int64) (written int64, err error) {
	written, cpErr := io.CopyN(dst, rc, n)

	var rcErr, dstErr error
	if rc != nil {
		rcErr = rc.Close()
	}
	if dst != nil {
		dstErr = dst.Close()
	}

	if cpErr != nil {
		err = errors.Join(err, fmt.Errorf("copying file: %w", cpErr))
	}
	if rcErr != nil {
		err = errors.Join(err, fmt.Errorf("closing stream: %w", rcErr))
	}
	if dstErr != nil {
		err = errors.Join(err, fmt.Errorf("flushing destination: %w", dstErr))
	}

	return written, err
}

func compSize(f *zip.File) int64 {
	if f == nil {
		return -1
	}
	n := int64(math.MaxInt64)
	if f.CompressedSize64 < math.MaxInt64 {
		n = int64(f.CompressedSize64)
	}
	return n
}

func zipTimes(f *zip.File, path string) error {
	if f == nil {
		return nil
	}
	mtime := f.Modified
	if mtime.IsZero() {
		mtime = time.Now()
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		return fmt.Errorf("zip times %w", err)
	}
	return nil
}

// ZipUnzip extracts the content of the src ZIP archive using the [unzip program].
// The format is credited to Phil Katz.
// If the targets are empty then all files are extracted.
//
// The use of [Extractor.Zip] is preferred over this method.
//
// [unzip program]: https://www.linux.org/docs/man1/unzip.html
func (x Extractor) ZipUnzip(ctx context.Context, targets ...string) error {
	const format = "extract unzip %s %w"
	const file = command.Unzip

	src, dst := x.Source, x.Destination
	if dst == "" {
		return ErrDest
	}

	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutExtract)
	defer cancel()

	// Info-ZIP unzip syntax:
	// unzip [-options] file[.zip] [file(s)...] [-x file(s)] [-d exdir]
	const (
		quieter        = "-qq" // quiet mode (suppress output and warnings)
		notimestamps   = "-DD" // skip restoration of directory timestamps
		allowCtrlChars = "-^"  // allow control characters in extracted filenames
		overwrite      = "-o"  // overwrite existing files without prompting
		targetDir      = "-d"  // target directory to extract files into
		stopSwitch     = "--"  // stop parsing options
	)

	const size = 7 + 2
	arg := make([]string, 0, size+len(targets))
	arg = append(arg, quieter, notimestamps, allowCtrlChars, overwrite, stopSwitch, src)
	arg = append(arg, targets...)
	arg = append(arg, targetDir, dst)

	return x.Run(ctx, file, prog, arg...)
}

// ZipHW extracts the content of the src ZIP archive using the [hwzip program].
// The format is credited to Phil Katz.
//
// Modern unzip only supports the Deflate and Store compression methods.
//
// hwzip supports these legacy PKZIP formats that are not supported anymore:
//   - Shrink
//   - Reduce
//   - Implode
//
// hwzip does not support targets, the extracting of individual files from a zip archive.
//
// [hwzip program]: https://www.hanshq.net/zip2.html
func (x Extractor) ZipHW(ctx context.Context) error {
	return x.Generic(ctx, Run{
		Program: command.HWZip,
		Extract: "extract",
	})
}
