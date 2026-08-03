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
	const format = "content zipinfo %w"
	prog, err := exec.LookPath(command.ZipInfo)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	const list = "-1"
	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	var buf bytes.Buffer
	cmd.Stderr = &buf
	out, err := cmd.Output()
	if err != nil {
		// handle broken zips that still contain some valid files
		if buf.String() != "" && len(out) > 0 {
			c.Files = strings.Split(string(out), "\n")
			c.Files = slices.DeleteFunc(c.Files, func(s string) bool {
				return strings.TrimSpace(s) == ""
			})
			c.Ext = zipx
			return nil
		}
		// otherwise the zipinfo threw an error
		return fmt.Errorf(format+": %s", err, src)
	}
	if len(out) == 0 {
		return ErrRead
	}
	c.Files = strings.Split(string(out), "\n")
	c.Files = slices.DeleteFunc(c.Files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	c.Ext = zipx
	return nil
}

// Zip extracts the content of the src ZIP archive.
// The format is credited to Phil Katz using the [unzip program].
// If the targets are empty then all files are extracted.
//
// [unzip program]: https://www.linux.org/docs/man1/unzip.html
func (x Extractor) Zip(ctx context.Context, targets ...string) error {
	const format = "extract unzip %w"
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Unzip)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	if dst == "" {
		return ErrDest
	}
	ctx, cancel := context.WithTimeout(ctx, command.TimeoutExtract)
	defer cancel()
	// [-options]
	const (
		test            = "-t"  // test archive files
		caseinsensitive = "-C"  // use case-insensitive matching
		notimestamps    = "-DD" // skip restoration of timestamps
		junkpaths       = "-j"  // junk paths, ignore directory structures
		overwrite       = "-o"  // overwrite existing files without prompting
		quiet           = "-q"  // quiet
		quieter         = "-qq" // quieter
		targetDir       = "-d"  // target directory to extract files to
		allowCtrlChars  = "-^"  // allow control characters in filenames
		zipArgsBase     = 5     // base args (quieter, notimestamps, allowCtrlChars, overwrite, src)
		zipArgsExtra    = 2     // extra args after targets (targetDir, dst)
	)
	// unzip [-options] file[.zip] [file(s)...] [-x files(s)] [-d exdir]
	// file[.zip]		path to the zip archive
	// [file(s)...]		optional list of archived files to process, sep by spaces.
	// [-x files(s)]	optional files to be excluded.
	// [-d exdir]		optional target directory to extract files in.
	args := make([]string, 0, zipArgsBase+len(targets)+zipArgsExtra)
	args = append(args, quieter, notimestamps, allowCtrlChars, overwrite, src)
	args = append(args, targets...)
	args = append(args, targetDir, dst)
	cmd := exec.CommandContext(ctx, prog, args...)
	var buf bytes.Buffer
	cmd.Stderr = &buf
	if err = cmd.Run(); err != nil {
		if buf.String() != "" {
			return fmt.Errorf(format+": %s: %s", ErrProg, prog, strings.TrimSpace(buf.String()))
		}
		return fmt.Errorf(format+": %s", err, prog)
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
