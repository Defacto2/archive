package archive

import (
	"bytes"
	"context"
	"errors"
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
func (c *Content) ARJ(ctx context.Context, src string) (err error) {
	const format = "content arj %w"
	prog, err := exec.LookPath(command.Arj)
	if err != nil {
		return fmt.Errorf(format, err)
	}

	newname := src
	name, err := HardLink(arjx, src)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	if name != "" {
		newname = name
		defer func() {
			if cErr := os.Remove(name); cErr != nil {
				err = errors.Join(err, fmt.Errorf(format, err))
			}
		}()
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const verboselist = "l"
	cmd := exec.CommandContext(ctx, prog, verboselist, newname)
	var buf bytes.Buffer
	cmd.Stderr = &buf
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf(format, err)
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
	const match = "is not an ARJ archive"
	return bytes.Contains(output, []byte(match))
}

// ARJ extracts the targets from the source ARJ archive
// to the destination directory using the [arj program].
// If the targets are empty then all files are extracted.
//
// [arj program]: https://arj.sourceforge.net/
func (x Extractor) ARJ(ctx context.Context, targets ...string) error {
	const format = "extract arj %w"
	src, dst := x.Source, x.Destination
	if inf, err := os.Stat(dst); err != nil {
		return fmt.Errorf("%w: %s", err, dst)
	} else if !inf.IsDir() {
		return fmt.Errorf("%w: %s", ErrPath, dst)
	}
	// note: only use arj, as unarj offers limited functionality
	prog, err := exec.LookPath(command.Arj)
	if err != nil {
		return fmt.Errorf(format, err)
	}

	newname := src
	name, err := HardLink(arjx, src)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	if name != "" {
		newname = name
		defer func() {
			if cErr := os.Remove(name); cErr != nil {
				err = errors.Join(err, fmt.Errorf(format, err))
			}
		}()
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutDefunct)
	defer cancel()
	// note: these flags are for arj32 v3.10
	const (
		extract      = "x"   // x extract files
		yes          = "-y"  // -y assume yes to all queries
		targetDir    = "-ht" // -ht target directory
		arjArgsBase  = 3     // base number of args before targets and targetDir
		arjArgsExtra = 1     // extra args after targets (targetDir+dst)
	)
	args := make([]string, arjArgsBase, arjArgsBase+len(targets)+arjArgsExtra)
	args[0] = extract
	args[1] = yes
	args[2] = newname
	args = append(args, targets...)
	args = append(args, targetDir+dst)
	cmd := exec.CommandContext(ctx, prog, args...)
	var buf bytes.Buffer
	cmd.Stderr = &buf
	if err = cmd.Run(); err != nil {
		if buf.String() != "" {
			return fmt.Errorf(format+": %s: '%s'",
				ErrProg, prog, strings.TrimSpace(buf.String()))
		}
		return fmt.Errorf(format+": %s", err, prog)
	}
	return nil
}
