package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file zip7.go contains the 7-Zip compression methods.

// Zip7 returns the content of the src 7-zip archive.
// The format credited to Igor Pavlov and using the [7z program].
//
// On some Linux distributions the 7z program is named 7zz.
// The legacy version of the 7z program, the p7zip package
// should not be used!
//
// [7z program]: https://7-zip.org/
func (c *Content) Zip7(ctx context.Context, src string) error {
	const format = "content 7zip %s %w"
	const file = command.Zip7
	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const list = "l"
	const stopParsing = "--" // prevent files named with "-" from being parsed as flags
	out, err := c.Run(ctx, file, prog, list, stopParsing, src)
	if err != nil {
		return err
	}

	if not7zip(out) {
		return ErrRead
	}

	c.Files = Zip7s(out)
	c.Ext = zip7x
	return nil
}

// Zip7s parses the output of the 7z list command and returns the listed filenames.
func Zip7s(out []byte) []string {
	var files []string
	listTable := false
	listIndex := -1

	for s := range bytes.Lines(out) {
		line := string(s)
		trimmed := strings.TrimSpace(line)

		// find the table header line and the exact column start of "Name"
		const substr = "Name"
		if strings.HasPrefix(trimmed, "Date") && strings.Contains(trimmed, substr) {
			listIndex = strings.Index(line, substr)
			continue
		}

		const prefix = "-------------------"
		if strings.HasPrefix(trimmed, prefix) {
			if !listTable && listIndex != -1 {
				listTable = true // entering file list
			} else if listTable {
				break // at the end of the separator line
			}
			continue
		}

		// parse the table rows
		if listTable && listIndex != -1 {
			if len(line) <= listIndex {
				continue
			}

			// skip directory entries (marked with 'D' in Attr column)
			// the attr column is typically between index 20 and 25
			const directory = "D"
			if len(line) >= 25 && strings.Contains(line[20:25], directory) {
				continue
			}

			name := strings.TrimRight(line[listIndex:], "\r\n")
			if name != "" {
				files = append(files, name)
			}
		}
	}

	return files
}

// not7zip returns true if the output is not a 7z archive.
// The 7zz application supports many archive formats but in this
// case we are only interested in the 7z format.
func not7zip(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	return !bytes.Contains(output, []byte("Type = 7z"))
}

// Zip7 extracts the targets from the source 7z archive
// to the destination directory using the [7z program].
// If the targets are empty then all files are extracted.
//
// On some Linux distributions the 7z program is named 7zz.
// The legacy version of the 7z program, the p7zip package
// should not be used!
//
// [7z program]: https://www.7-zip.org/
func (x Extractor) Zip7(ctx context.Context, targets ...string) error {
	const format = "extract 7z %s %w"
	const file = command.Zip7

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

	const (
		extract     = "e"    // e extracts files
		extractFull = "x"    // x extracts files with full directory paths
		overwrite   = "-aoa" // -aoa overwrite all existing files
		quiet       = "-bb0" // -bb0 quiet mode
		targetDir   = "-o"   // -o output directory
		yes         = "-y"   // -y assume yes on all queries
		stopSwitch  = "--"   // -- stop parsing switches [required]
	)
	const size = 7
	arg := make([]string, 0, size+len(targets))
	arg = append(arg, extractFull, overwrite, quiet, yes, targetDir+dst, stopSwitch, src)
	arg = append(arg, targets...)

	return x.Run(ctx, file, prog, arg...)
}
