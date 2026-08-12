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
	const format = "content arj %s %w"

	prog, err := exec.LookPath(command.Arj)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	newname := src
	name, err := HardLink(arjx, src)
	if err != nil {
		return fmt.Errorf(format, "hard link", err)
	}
	if name != "" {
		newname = name
		defer func() {
			if cErr := os.Remove(name); cErr != nil && !errors.Is(cErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf(format, "remove temp hardlink", cErr))
			}
		}()
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	// arj syntax: arj l <archive.arj>
	const verboselist = "l"
	cmd := exec.CommandContext(ctx, prog, verboselist, newname)

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

	if notArj(out) {
		return ErrRead
	}

	c.Ext = arjx
	c.Files = ARJs(out)
	return nil
}

// ARJs parses the output of the arj list command and returns the listed filenames.
func ARJs(out []byte) []string {
	var files []string
	listTable := false

	for line := range bytes.Lines(out) {
		trimmed := strings.TrimSpace(string(line))

		const filename, original = "Filename", "Original"
		if strings.HasPrefix(trimmed, filename) && strings.Contains(trimmed, original) {
			continue
		}

		const prefix = "------------"
		if strings.HasPrefix(trimmed, prefix) {
			if !listTable {
				listTable = true
				continue
			}
			// a second prefix separator indicates the eof list
			break
		}

		if listTable {
			const field = 14
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
func (x Extractor) ARJ(ctx context.Context, targets ...string) error {
	const format = "extract arj %s %w"

	src, dst := x.Source, x.Destination
	if dst == "" {
		return ErrDest
	}
	const perm = 0o755
	if err := os.MkdirAll(dst, perm); err != nil {
		return fmt.Errorf(format, "mkdir dst "+dst, err)
	}

	// note: only use arj, as unarj offers limited functionality
	prog, err := exec.LookPath(command.Arj)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	newname := src
	name, err := HardLink(arjx, src)
	if err != nil {
		return fmt.Errorf(format, "hard link", err)
	}
	if name != "" {
		newname = name
		defer func() {
			if cErr := os.Remove(name); cErr != nil && !errors.Is(cErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf(format, "hard link cleanup", err))
			}
		}()
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutDefunct)
	defer cancel()

	// note: these flags are for arj32 v3.10
	const (
		extract   = "x"   // x extract files
		yes       = "-y"  // -y assume yes to all queries
		targetDir = "-ht" // -ht target directory
	)

	const size = 4
	arg := make([]string, 0, size+len(targets))
	arg = append(arg, extract, yes, newname)
	arg = append(arg, targets...)
	arg = append(arg, targetDir+dst)

	return x.Run(ctx, "arj", prog, arg...)
}
