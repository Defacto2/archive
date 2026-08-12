package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"unicode"

	"github.com/Defacto2/archive/command"
)

// Package file lha.go contains the LHA/LZH compression methods.

// LHA returns the content of the src LHA or LZH archive.
// The format credited to Haruyasu Yoshizaki (Yoshi) using the [lha program].
//
// On Linux either the jlha-utils or lhasa work.
//
// [lha program]: https://fragglet.github.io/lhasa/
func (c *Content) LHA(ctx context.Context, src string) error {
	const format = "content lha %s %w"
	const file = command.Lha
	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const list = "-l"
	out, err := c.Run(ctx, file, prog, list, src)
	if err != nil {
		return err
	}

	if notLHA(out) {
		return ErrRead
	}

	c.Files = LHAs(out)
	c.Ext = lhax
	return nil
}

// LHAs parses the output of the lha list command and returns the listed filenames.
func LHAs(out []byte) []string {
	var files []string
	listTable := false
	listIndex := -1

	for s := range bytes.Lines(out) {
		line := string(s)
		trimmed := strings.TrimSpace(line)

		const prefix = "----------"
		if strings.HasPrefix(trimmed, prefix) {
			if !listTable {
				// top border
				listTable = true
				listIndex = strings.LastIndex(line, " ") + 1
				continue
			}
			// bottom border
			break
		}

		if !listTable || trimmed == "" {
			continue
		}

		if fallback := listIndex > 0 && len(line) > listIndex; fallback {
			file := strings.TrimSpace(line[listIndex:])
			if file != "" {
				files = append(files, file)
			}
		}
	}

	return files
}

// notLHA returns true if the output is not an LHA archive.
func notLHA(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	var clean strings.Builder
	for _, r := range string(output) {
		if !unicode.IsSpace(r) {
			clean.WriteRune(r)
		}
	}
	s := clean.String()

	// match "Total 0 files 0" regardless of space count or types
	if strings.Contains(s, "Total0files0") {
		return true
	}

	return false
}

// LHA extracts the targets from the source LHA/LZH archive.
// The format credited to Haruyasu Yoshizaki (Yoshi) using the [lha program].
// If the targets are empty then all files are extracted.
//
// On Linux either the jlha-utils or lhasa work.
//
// [lha program]: https://fragglet.github.io/lhasa/
func (x Extractor) LHA(ctx context.Context, targets ...string) error {
	const format = "extract lha %s %w"
	const file = command.Lha

	src, dst := x.Source, x.Destination
	if dst == "" {
		return ErrDest
	}
	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutDefunct)
	defer cancel()

	// example command: lha -eq2w=destdir/ archive *
	const (
		extract     = "e" // extract from archive (x also does  the same)
		ignorepaths = "i" // ignore directory path
		overwrite   = "f" // force overwrite (no prompt)
	)

	const size = 3
	arg := make([]string, 0, size+len(targets))

	param := func() string {
		const format = "-%s%s%sw=%s"
		return fmt.Sprintf(format, extract, overwrite, ignorepaths, dst)
	}
	arg = append(arg, param(), src)

	// convert and append targets as lowercase, which is a quirk with the lha program
	for _, target := range targets {
		arg = append(arg, strings.ToLower(target))
	}

	return x.Run(ctx, file, prog, arg...)
}
