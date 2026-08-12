package archive

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
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
func (c *Content) Rar(ctx context.Context, src string) error {
	const format = "content unrar %s %w"
	const file = command.Unrar
	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const listBare = "lb"
	const excludePaths = "-ep"
	const noComments = "-c-"
	out, err := c.Run(ctx, file, prog, listBare, excludePaths, noComments, src)
	if err != nil {
		return err
	}

	if len(out) == 0 {
		return ErrRead
	}

	for line := range strings.Lines(string(out)) {
		if name := strings.TrimSpace(line); name != "" {
			c.Files = append(c.Files, name)
		}
	}

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
func (x Extractor) Rar(ctx context.Context, targets ...string) error {
	return x.rar(ctx, true, targets...)
}

// RarWithoutPaths extracts the targets from the source RAR archive
// to the destination directory using the [unrar program].
// If the targets are empty then all files are extracted.
//
// It uses the "extract files without archived paths" and the "exclude
// paths from names" flags.
//
// [unrar program]: https://www.rarlab.com/rar_add.htm
func (x Extractor) RarWithoutPaths(ctx context.Context, targets ...string) error {
	return x.rar(ctx, false, targets...)
}

func (x Extractor) rar(ctx context.Context, withFullPaths bool, targets ...string) error {
	const format = "extract unrar %w"
	const file = command.Unrar

	src, dst := x.Source, x.Destination
	if dst == "" {
		return ErrDest
	}

	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutExtract)
	defer cancel()

	const (
		eXtract        = "x"   // x extract files with full path (cannot use with -ep)
		extractNoPaths = "e"   // extract files (for use with -ep)
		noPaths        = "-ep" // -ep do not preserve paths
		noComments     = "-c-" // -c- do not display comments
		rename         = "-or" // -or rename files automatically
		yes            = "-y"  // -y assume yes to all queries
		outputPath     = "-op" // -op output path
	)

	// unrar -op requires the destination directory end with a separator
	if !strings.HasSuffix(dst, string(filepath.Separator)) {
		dst += string(filepath.Separator)
	}

	size := 6
	if !withFullPaths {
		size = 7
	}
	arg := make([]string, 0, size+len(targets))

	if !withFullPaths {
		// extract without paths (size = 7)
		arg = append(arg, extractNoPaths, noPaths, noComments, rename, yes, src)
	} else {
		// extract full path (size = 6)
		arg = append(arg, eXtract, noComments, rename, yes, src)
	}
	arg = append(arg, targets...)
	arg = append(arg, outputPath+dst)

	return x.Run(ctx, file, prog, arg...)
}
