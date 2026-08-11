package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file arc.go contains the ARC archive compression methods.

// ARC returns the content of the src ARC archive.
// The format once credited to System Enhancement Associates,
// but now using the [arc program] by Howard Chu.
//
// [arc program]: https://github.com/hyc/arc
func (c *Content) ARC(ctx context.Context, src string) error {
	const format = "context arc %s %w"
	prog, err := exec.LookPath(command.Arc)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const list = "l"
	cmd := exec.CommandContext(ctx, prog, list, src)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf(format, "timeout", ctx.Err())
		}

		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return fmt.Errorf(format, "exec "+src+": "+stderrStr, err)
		}
		return fmt.Errorf(format, "exec "+src, err)
	}

	if notArc(out) {
		return ErrRead
	}

	c.Files = ARCs(out)
	c.Ext = arcx
	return nil
}

// ARCs parses the output of the arc list command and returns the listed filenames.
func ARCs(out []byte) []string {
	var files []string
	listTable := false

	for line := range bytes.Lines(out) {
		trimmed := strings.TrimSpace(string(line))

		const name, length = "Name", "Length"
		if strings.HasPrefix(trimmed, name) && strings.Contains(trimmed, length) {
			continue
		}

		const prefix = "============"
		if strings.HasPrefix(trimmed, prefix) {
			listTable = true
			continue
		}

		// the footer line with "====" is the end of file entries
		const eof, total = "====", "Total"
		if strings.HasPrefix(trimmed, eof) || strings.HasPrefix(trimmed, total) {
			break
		}

		if listTable {
			const field = 12
			if len(line) < field {
				continue
			}

			name := strings.TrimSpace(string(line[:field]))
			if name != "" {
				files = append(files, name)
			}
		}
	}

	return files
}

// notArc returns true if the output indicates the file is not a valid ARC archive.
func notArc(output []byte) bool {
	if len(output) == 0 {
		return true
	}

	b := bytes.ToLower(output)

	// Howard Chu's arc outputs error messages containing "bad header" or "not an arc"
	return bytes.Contains(b, []byte("bad header")) ||
		bytes.Contains(b, []byte("not an arc")) ||
		bytes.Contains(b, []byte("archive format error"))
}

// ARC extracts the content of the ARC archive.
// The format once credited to System Enhancement Associates,
// but now using the [arc program] by Howard Chu.
// If the targets are empty then all files are extracted.
//
// [arc program]: https://github.com/hyc/arc
func (x Extractor) ARC(ctx context.Context, targets ...string) error {
	return x.Generic(ctx, Run{
		Program: command.Arc,
		Extract: "x",
	}, targets...)
}
