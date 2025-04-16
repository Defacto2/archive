package archive

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file arj.go contains the ARJ compression methods.

// ARJ returns the content of the src ARJ archive.
// The format credited to Robert Jung using the [arj program].
//
// [arj program]: https://arj.sourceforge.net/
func (c *Content) ARJ(src string) error {
	prog, err := exec.LookPath(command.Arj)
	if err != nil {
		return fmt.Errorf("content arj %w", err)
	}

	newname := src
	if name, err := HardLink(arjx, src); err != nil {
		return fmt.Errorf("content arj %w", err)
	} else if name != "" {
		newname = name
		defer os.Remove(name)
	}

	const verboselist = "l"
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, verboselist, newname)
	cmd.Stderr = &buf
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("content arj %w", err)
	}
	if notArj(out) {
		return ErrRead
	}
	c.Ext = arjx
	c.Files = arjFiles(out)
	return nil
}

// arjFiles parses the output of the arj list command and returns the listed filenames.
func arjFiles(out []byte) []string {
	// Filename       Original Compressed Ratio DateTime modified Attributes/GUA BPMGS
	// ------------ ---------- ---------- ----- ----------------- -------------- -----
	// TESTDAT1.TXT       2009        889 0.443 25-02-14 13:21:10                  1
	// TESTDAT2.TXT        469        266 0.567 25-02-14 13:17:34                  1
	// TESTDAT3.TXT      81410      22438 0.276 25-02-14 13:21:02                  1
	// ------------ ---------- ---------- -----
	//      3 files      83888      23593 0.281

	const tableEnd = 2
	skip1 := []byte("Filename       Original")
	skip2 := []byte("------------ ----------")
	files := []string{}
	skipped := 0
	for line := range bytes.Lines(out) {
		if bytes.HasPrefix(line, skip1) {
			skipped++
			continue
		}
		if bytes.HasPrefix(line, skip2) {
			skipped++
			continue
		}
		if skipped == 0 {
			continue
		}
		if skipped > tableEnd {
			return files
		}
		file := string(line[0:12])
		files = append(files, file)
	}
	return files
}

// notArj returns true if the output is not an ARJ archive.
func notArj(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	return bytes.Contains(output, []byte("is not an ARJ archive"))
}

// ARJ extracts the targets from the source ARJ archive
// to the destination directory using the [arj program].
// If the targets are empty then all files are extracted.
//
// [arj program]: https://arj.sourceforge.net/
func (x Extractor) ARJ(targets ...string) error {
	src, dst := x.Source, x.Destination
	if inf, err := os.Stat(dst); err != nil {
		return fmt.Errorf("%w: %s", err, dst)
	} else if !inf.IsDir() {
		return fmt.Errorf("%w: %s", ErrPath, dst)
	}
	// note: only use arj, as unarj offers limited functionality
	prog, err := exec.LookPath(command.Arj)
	if err != nil {
		return fmt.Errorf("extract arj %w", err)
	}

	newname := src
	if name, err := HardLink(arjx, src); err != nil {
		return fmt.Errorf("extract arj %w", err)
	} else if name != "" {
		newname = name
		defer os.Remove(name)
	}

	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutDefunct)
	defer cancel()
	// note: these flags are for arj32 v3.10
	const (
		extract   = "x"   // x extract files
		yes       = "-y"  // -y assume yes to all queries
		targetDir = "-ht" // -ht target directory
	)
	args := []string{extract, yes, newname}
	args = append(args, targets...)
	args = append(args, targetDir+dst)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &buf
	if err = cmd.Run(); err != nil {
		if buf.String() != "" {
			return fmt.Errorf("extract arj %w: %s: %q",
				ErrProg, prog, strings.TrimSpace(buf.String()))
		}
		return fmt.Errorf("extract arj %w: %s", err, prog)
	}
	return nil
}
