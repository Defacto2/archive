package archive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Defacto2/archive/command"
	"github.com/Defacto2/helper"
)

// Run holds the program and extract command for use with the generic extractor.
type Run struct {
	Program string // Program is the archiver program to run, but not the full path.
	Extract string // Extract is the program command to extract files from the archive.
}

// Generic extracts the targets from the source archive
// to the destination directory using the specified archive program.
// If the targets are empty then all files are extracted.
//
// It is used for archive formats that are not widely supported
// or have a limited feature set including ARC, HWZIP, and others.
//
// These DOS era archive formats are not widely supported.
// They also does not support extracting to a target directory.
// To work around this, Generic copies the source archive
// to the destination directory, uses that as the working directory
// and extracts the files. The copied source archive is then removed.
func (x Extractor) Generic(ctx context.Context, run Run, targets ...string) (err error) {
	const format = "generic archive"
	name := run.Program
	src, dst := x.Source, x.Destination
	if inf, err := os.Stat(dst); err != nil {
		return fmt.Errorf("%w: %s", err, dst)
	} else if !inf.IsDir() {
		return fmt.Errorf("%w: %s", ErrPath, dst)
	}

	prog, err := exec.LookPath(run.Program)
	if err != nil {
		return fmt.Errorf(format+" %s extract %w", name, err)
	}

	srcInDst := filepath.Join(dst, filepath.Base(src))
	if _, err := helper.Duplicate(src, srcInDst); err != nil {
		return fmt.Errorf(format+" %s duplicate %w", name, err)
	}
	defer func() {
		if cErr := os.Remove(srcInDst); cErr != nil {
			err = errors.Join(err, fmt.Errorf(format+" %w", cErr))
		}
	}()

	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(ctx, command.TimeoutDefunct)
	defer cancel()
	const size = 2
	args := make([]string, size, size+len(targets))
	args[0] = run.Extract
	args[1] = filepath.Base(src)
	args = append(args, targets...)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Dir = dst
	cmd.Stderr = &buf
	if err = cmd.Run(); err != nil {
		if buf.String() != "" {
			return fmt.Errorf(format+" %s %w: %s: '%s'", name,
				ErrProg, prog, strings.TrimSpace(buf.String()))
		}
		return fmt.Errorf(format+" %s %w: %s", name, err, prog)
	}
	return nil
}
