package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/Defacto2/archive/command"
)

// Package file arc.go contains the ARC archive compression methods.

// ARC returns the content of the src ARC archive, once credited to System Enhancement Associates,
// but now using the [arc port] by Howard Chu.
//
// [arc program]: https://github.com/hyc/arc
func (c *Content) ARC(src string) error {
	prog, err := exec.LookPath(command.Arc)
	if err != nil {
		return fmt.Errorf("archive arc reader %w", err)
	}
	const list = "l"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("archive arc output %w", err)
	}
	if arcEmpty(out) {
		return ErrRead
	}
	c.Files = arcFiles(out)
	c.Ext = arcx
	return nil
}

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

// arcEmpty returns true if the output from an ARC list shows an empty or unsupported archive.
func arcEmpty(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	p := bytes.ReplaceAll(output, []byte("  "), []byte(""))
	return bytes.Contains(p, []byte("has a bad header"))
}

// ARC extracts the targets from the source ARC archive
// to the destination directory using the [arc program].
// If the targets are empty then all files are extracted.
//
// [arc program]: https://github.com/hyc/arc
func (x Extractor) ARC(targets ...string) error {
	return x.Generic(Run{
		Program: command.Arc,
		Extract: "x",
	}, targets...)
}
