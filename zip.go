package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file zip.go contains the ZIP compression methods.

// Zip returns the content of the src ZIP archive.
// The format is credited to Phil Katz using the [zipinfo program].
//
// [zipinfo program]: https://infozip.sourceforge.net/
func (c *Content) Zip(ctx context.Context, src string) error {
	const format = "content zipinfo %s %w"
	prog, err := exec.LookPath(command.ZipInfo)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const list = "-1"
	const stopParsing = "--"
	cmd := exec.CommandContext(ctx, prog, list, stopParsing, src)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf(format, "timeout", ctx.Err())
		}

		// handle broken ZIPs that still returned partial file listings
		if stderrBuf.Len() > 0 && len(out) > 0 {
			c.Files = ZipInfo(out)
			c.Ext = zipx
			return nil
		}

		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return fmt.Errorf(format, "exec "+src+": "+stderrStr, err)
		}
		return fmt.Errorf(format, "exec "+src, err)
	}

	if len(out) == 0 {
		return ErrRead
	}

	c.Files = ZipInfo(out)
	c.Ext = zipx
	return nil
}

// ZipInfo cleans and splits raw zipinfo -1 output into a slice of filenames.
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
// The format is credited to Phil Katz using the [unzip program].
// If the targets are empty then all files are extracted.
//
// [unzip program]: https://www.linux.org/docs/man1/unzip.html
func (x Extractor) Zip(ctx context.Context, targets ...string) error {
	const format = "extract unzip %s %w"

	src, dst := x.Source, x.Destination
	if dst == "" {
		return ErrDest
	}

	prog, err := exec.LookPath(command.Unzip)
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

	cmd := exec.CommandContext(ctx, prog, arg...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf(format, "timeout", ctx.Err())
		}

		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return fmt.Errorf(format, "exec: "+stderrStr, err)
		}
		return fmt.Errorf(format, "exec", err)
	}

	return nil
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
