package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/Defacto2/archive/command"
)

// Package file arc.go contains the ARC archive compression methods.

// ARC returns the content of the src ARC archive.
// The format once credited to System Enhancement Associates,
// but now using the [arc program] by Howard Chu.
//
// [arc program]: https://github.com/hyc/arc
func (c *Content) ARC(src string) error {
	prog, err := exec.LookPath(command.Arc)
	if err != nil {
		return fmt.Errorf("content arc %w", err)
	}
	const list = "l"
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &buf
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("content arc %w", err)
	}
	if notArc(out) {
		return ErrRead
	}
	c.Files = arcFiles(out)
	c.Ext = arcx
	return nil
}

// arcFiles parses the output of the arc list command and returns the listed filenames.
func arcFiles(out []byte) []string {
	// Name          Length    Date
	// ============  ========  =========
	// TESTDAT1.TXT      2009  14 Feb 25
	// TESTDAT2.TXT       469  14 Feb 25
	// TESTDAT3.TXT     81410  14 Feb 25
	// 		====  ========
	// Total      3     83888

	skip1 := []byte("Name          Length    Date")
	skip2 := []byte("============  ========  =========")
	end := []byte("====  ========")
	files := []string{}
	for line := range bytes.Lines(out) {
		if bytes.HasPrefix(line, skip1) {
			continue
		}
		if bytes.HasPrefix(line, skip2) {
			continue
		}
		if bytes.HasPrefix(bytes.TrimSpace(line), end) {
			return files
		}
		file := string(line[0:12])
		files = append(files, file)
	}
	return files
}

// notArc returns true if the output is not an ARC archive.
func notArc(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	p := bytes.ReplaceAll(output, []byte("  "), []byte(""))
	return bytes.Contains(p, []byte("has a bad header"))
}

// ARC extracts the content of the ARC archive.
// The format once credited to System Enhancement Associates,
// but now using the [arc program] by Howard Chu.
// If the targets are empty then all files are extracted.
//
// [arc program]: https://github.com/hyc/arc
func (x Extractor) ARC(targets ...string) error {
	return x.Generic(Run{
		Program: command.Arc,
		Extract: "x",
	}, targets...)
}
