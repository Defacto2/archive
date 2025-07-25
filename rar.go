package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file rar.go contains the RAR compression methods.

// Rar returns the content of the src RAR archive.
// The format is credited to Alexander Roshal using the [unrar program].
//
// On Linux there are two versions of the unrar program, the freeware
// version by Alexander Roshal and the feature incomplete [unrar-free].
// The freeware version is the recommended program for extracting RAR archives.
//
// [unrar program]: https://www.rarlab.com/rar_add.htm
func (c *Content) Rar(src string) error {
	prog, err := exec.LookPath(command.Unrar)
	if err != nil {
		return fmt.Errorf("content unrar %w", err)
	}
	const (
		listBrief  = "lb"
		noComments = "-c-"
	)
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), command.TimeoutList)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, listBrief, "-ep", noComments, src)
	cmd.Stderr = &buf
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("content unrar %w: %s", err, src)
	}
	if len(out) == 0 {
		return ErrRead
	}
	c.Files = strings.Split(string(out), "\n")
	c.Files = slices.DeleteFunc(c.Files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	c.Ext = rarx
	return nil
}

// Rar extracts the targets from the source RAR archive
// to the destination directory using the [unrar program].
// If the targets are empty then all files are extracted.
//
// On Linux there are two versions of the unrar program, the freeware
// version by Alexander Roshal and the feature incomplete [unrar-free].
// The freeware version is the recommended program for extracting RAR archives.
//
// [unrar program]: https://www.rarlab.com/rar_add.htm
func (x Extractor) Rar(targets ...string) error {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Unrar)
	if err != nil {
		return fmt.Errorf("extract unrar %w", err)
	}
	if dst == "" {
		return ErrDest
	}
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), command.TimeoutExtract)
	defer cancel()
	const (
		eXtract    = "x"   // x extract files with full path
		noPaths    = "-ep" // -ep do not preserve paths
		noComments = "-c-" // -c- do not display comments
		rename     = "-or" // -or rename files automatically
		yes        = "-y"  // -y assume yes to all queries
		outputPath = "-op" // -op output path
	)
	args := []string{eXtract, noPaths, noComments, rename, yes, src}
	args = append(args, targets...)
	args = append(args, outputPath+dst)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &buf
	if err = cmd.Run(); err != nil {
		if buf.String() != "" {
			return fmt.Errorf("extract unrar %w: %s: %s", ErrProg, prog, strings.TrimSpace(buf.String()))
		}
		return fmt.Errorf("extract unrar %w: %s", err, prog)
	}
	return nil
}
