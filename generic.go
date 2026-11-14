package archive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Defacto2/archive/command"
)

var ErrNoFeedback = errors.New("command failed but it gave no feedback")

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
// or have a limited feature set including ARC, CAB, and others.
//
// These DOS-era archive formats do not support a destination directory flag.
// Generic works around this by setting the command working directory
// to the destination directory and passing the absolute path to the source archive.
func (x Extractor) Generic(ctx context.Context, run Run, targets ...string) error {
	src, dst := x.Source, x.Destination
	inf, sErr := os.Stat(dst)
	if sErr != nil {
		return fmt.Errorf("%w: %s", sErr, dst)
	}
	if !inf.IsDir() {
		return fmt.Errorf("%w: %s", ErrPath, dst)
	}

	name := run.Program
	prog, pErr := exec.LookPath(run.Program)
	if pErr != nil {
		return fmt.Errorf("generic archive %s extract: %w", name, pErr)
	}

	absSrc, aErr := filepath.Abs(src)
	if aErr != nil {
		return fmt.Errorf("generic archive %s abs: %w", name, aErr)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutDefunct)
	defer cancel()

	const size = 2
	args := make([]string, 0, size+len(targets))
	args = append(args, run.Extract, absSrc)
	args = append(args, targets...)

	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Dir = dst
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	rErr := cmd.Run()
	if rErr == nil {
		return nil
	}

	if s := truncate(stderr.Bytes()); s != "" {
		return fmt.Errorf("generic archive %s %w: %s with debug: '%s'",
			name, ErrProg, prog, s)
	}
	if s := truncate(stdout.Bytes()); s != "" {
		return fmt.Errorf("generic archive %s %w: %s with output: '%s'",
			name, ErrProg, prog, s)
	}
	return fmt.Errorf("generic archive %s %w: %s", name, ErrNoFeedback, prog)
}

func truncate(b []byte) string {
	const limit = 1000
	b = bytes.TrimSpace(b)
	if len(b) > limit {
		return string(b[:limit]) + "... (truncated)"
	}
	return string(b)
}
