package archive

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Defacto2/archive/command"
	"github.com/Defacto2/archive/expand"
	"github.com/Defacto2/archive/explode"
	"github.com/Defacto2/archive/sanitize"
	"github.com/Defacto2/archive/unshrink"
)

// Package file zip.go contains the ZIP package and compression methods.

type Compression uint16

const (
	Store Compression = iota
	Shrink
	Reduce1
	Reduce2
	Reduce3
	Reduce4
	Implode
	Unused
	Deflate
)

func init() { //nolint:gochecknoinits
	// one time registation to enable archive/zip to handle shrink, reduce, and implode compression methods
	unshrink.Register()
	expand.Register()
	explode.Register()
}

func (c Compression) String() string {
	names := [9]string{
		Store:   "Store",
		Shrink:  "Shrink",
		Reduce1: "Reduce1",
		Reduce2: "Reduce2",
		Reduce3: "Reduce3",
		Reduce4: "Reduce4",
		Implode: "Implode",
		Unused:  "unused",
		Deflate: "Deflate",
	}

	if int(c) >= len(names) {
		return fmt.Sprintf("Compression(%d)", c)
	}
	return names[c]
}

// Zip returns the content of the src ZIP archive.
// The format is credited to Phil Katz.
func (c *Content) Zip(ctx context.Context, src string) error {
	const format = "zip contents %s %w"

	if err := ctx.Err(); err != nil {
		return fmt.Errorf(format, "timeout", err)
	}

	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf(format, "open reader", err)
	}
	defer r.Close()

	c.Files = slices.Grow(c.Files, len(r.File))
	for _, f := range r.File {
		c.Files = append(c.Files, f.Name)
	}
	c.Ext = zipx
	return nil
}

// Zip extracts the content of the src ZIP archive.
// The format is credited to Phil Katz.
// If the targets are empty then all files are extracted.
//
// Individual file extraction errors are ignored, instead use [Extractor.ZipWithLogger].
func (x Extractor) Zip(ctx context.Context, targets ...string) error {
	logger := slog.New(slog.DiscardHandler)
	rc, err := x.zipReadCloser(logger)
	if err != nil {
		return err
	}
	defer rc.Close()
	return x.zipSkipErrors(ctx, logger, rc, targets...)
}

// ZipWithLogger extracts the content of the src ZIP archive.
// The format is credited to Phil Katz.
// If the targets are empty then all files are extracted.
//
// Individual file extraction errors are logged by the logger.
func (x Extractor) ZipWithLogger(ctx context.Context, logger *slog.Logger, targets ...string) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	rc, err := x.zipReadCloser(logger)
	if err != nil {
		return err
	}
	defer rc.Close()
	return x.zipSkipErrors(ctx, logger, rc, targets...)
}

// ZipStrict extracts the content of the src ZIP archive.
// The format is credited to Phil Katz.
// If the targets are empty then all files are extracted.
//
// The process is aborted and an error is returned for any
// file extraction errors and CRC32 validation checks.
func (x Extractor) ZipStrict(ctx context.Context, targets ...string) error {
	logger := slog.New(slog.DiscardHandler)
	rc, err := x.zipReadCloser(logger)
	if err != nil {
		return err
	}
	defer rc.Close()
	return x.zipStrict(ctx, rc, targets...)
}

func (x Extractor) zipReadCloser(logger *slog.Logger) (*zip.ReadCloser, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	const msg = "extract zip"
	const format = msg + " %s %w"

	if x.Destination == "" {
		logger.Error(msg + " extractor destination is empty")
		return nil, ErrDest
	}

	rc, err := zip.OpenReader(x.Source)
	if err != nil {
		logger.Error(msg+" cannot open extractor source",
			slog.String("source", x.Source), slog.Any("error", err))
		return nil, fmt.Errorf(format, "open source", err)
	}
	return rc, nil
}

func (x Extractor) zipSkipErrors( //nolint:funlen
	ctx context.Context, logger *slog.Logger, rc *zip.ReadCloser, targets ...string,
) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if rc == nil {
		return nil
	}

	const msg = "extract zip"
	const format = msg + " %s %w"
	for n, f := range rc.File {
		logName := slog.Group(strconv.Itoa(n), slog.String("name", f.Name),
			slog.String("method", Compression(f.Method).String()))
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

		local := sanitize.Name(f.Name)
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
		dst, err := os.OpenFile(path, flag, zipPerm(f))
		if err != nil {
			logErr("cannot open destination", err)
			rc.Close()
			continue
		}

		n, err := zipCopier(dst, rc, f)
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

func (x Extractor) zipStrict(ctx context.Context, rc *zip.ReadCloser, targets ...string) error {
	if rc == nil {
		return nil
	}

	const format = "extract zip %s %w"
	for _, f := range rc.File {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			return fmt.Errorf(format, "timeout", err)
		default:
		}

		if skipName(f.Name, targets...) {
			continue
		}

		local := sanitize.Name(f.Name)
		path := filepath.Join(x.Destination, local)
		if f.FileInfo().IsDir() {
			// create directories directly without opening file handles
			if err := os.MkdirAll(path, DirWriteReadRead); err != nil {
				return fmt.Errorf(format, "mkdir", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), DirWriteReadRead); err != nil {
			return fmt.Errorf(format, "mkdir parent", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf(format, "open", err)
		}

		const flag = WriteRead
		dst, err := os.OpenFile(path, flag, zipPerm(f))
		if err != nil {
			rc.Close()
			return fmt.Errorf(format, "open dest", err)
		}

		_, err = zipCopier(dst, rc, f)
		if err != nil {
			return fmt.Errorf(format, "copy", err)
		}

		if err := zipTimes(f, path); err != nil {
			return fmt.Errorf(format, "set time", err)
		}
	}

	return nil
}

func zipCopier(dst *os.File, rc io.ReadCloser, f *zip.File) (written int64, err error) {
	if f == nil {
		return 0, nil
	}

	hasher := crc32.NewIEEE()
	validate := io.MultiWriter(dst, hasher)

	// Use io.Copy with io.LimitReader instead of io.CopyN. This allows for
	// CRC32 verification while also keeping the logic safe.
	n := zipSize(f)
	written, cErr := io.Copy(validate, io.LimitReader(rc, n))

	var rErr, dErr error
	if rc != nil {
		rErr = rc.Close()
	}
	if dst != nil {
		dErr = dst.Close()
	}

	if cErr != nil {
		err = errors.Join(err, fmt.Errorf("io copy file %w", cErr))
	}
	if rErr != nil {
		err = errors.Join(err, fmt.Errorf("close stream %w", rErr))
	}
	if dErr != nil {
		err = errors.Join(err, fmt.Errorf("flush destination %w", dErr))
	}

	sum32 := hasher.Sum32()
	if cErr == nil && !zipVerified(f, sum32) {
		const format = "zip archive crc32 does not match, '%X' vs '%X' %w"
		err = errors.Join(err, fmt.Errorf(format, f.CRC32, sum32, ErrCorruption))
	}
	return written, err
}

func zipPerm(f *zip.File) fs.FileMode {
	const def = WriteWriteRead
	if f == nil {
		return def
	}
	if mode := f.Mode(); mode != 0 {
		return mode
	}
	return def
}

func zipSize(f *zip.File) int64 {
	if f == nil {
		return -1
	}
	n := int64(math.MaxInt64)
	if f.UncompressedSize64 < math.MaxInt64 {
		n = int64(f.UncompressedSize64)
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

func zipVerified(f *zip.File, sum32 uint32) bool {
	if f == nil {
		return false
	}
	return f.CRC32 == sum32
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
