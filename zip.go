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
func (c *Content) Zip(src string) error {
	prog, err := exec.LookPath(command.ZipInfo)
	if err != nil {
		return fmt.Errorf("content zipinfo %w", err)
	}
	const list = "-1"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		// handle broken zips that still contain some valid files
		if b.String() != "" && len(out) > 0 {
			// return files, zipx, nil
			return nil
		}
		// otherwise the zipinfo threw an error
		return fmt.Errorf("content zipinfo %w: %s", err, src)
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
func (x Extractor) Zip(targets ...string) error {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Unzip)
	if err != nil {
		return fmt.Errorf("extract unzip %w", err)
	}
	if dst == "" {
		return ErrDest
	}
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutExtract)
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
	)
	// unzip [-options] file[.zip] [file(s)...] [-x files(s)] [-d exdir]
	// file[.zip]		path to the zip archive
	// [file(s)...]		optional list of archived files to process, sep by spaces.
	// [-x files(s)]	optional files to be excluded.
	// [-d exdir]		optional target directory to extract files in.
	args := []string{quieter, notimestamps, allowCtrlChars, overwrite, src}
	args = append(args, targets...)
	args = append(args, targetDir, dst)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return fmt.Errorf("extract unzip %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("extract unzip %w: %s", err, prog)
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
func (x Extractor) ZipHW() error {
	return x.Generic(Run{
		Program: command.HWZip,
		Extract: "extract",
	})
}
