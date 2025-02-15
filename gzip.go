package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Defacto2/helper"
)

// Package file gzip.go contains the Gzip compression methods.

// Gzip returns the uncompressed filename of the [gzip] archive which is expected to be a single file.
//
// [gzip]: https://www.gnu.org/software/gzip/
func (c *Content) Gzip(src string) error {
	prog, err := exec.LookPath("gzip")
	if err != nil {
		return fmt.Errorf("archive gzip reader %w", err)
	}
	const test = "-t"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, test, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("archive gzip output %w", err)
	}
	out = bytes.TrimSpace(out)
	if bytes.Contains(out, []byte("not in gzip format")) {
		return ErrRead
	}
	if len(out) == 0 {
		base := strings.ToLower(filepath.Base(src))
		if strings.HasSuffix(base, gzipx) {
			s := strings.Split(base, ".")
			name := strings.Join(s[:len(s)-1], ".")
			c.Files = append(c.Files, name)
			c.Ext = gzipx
		}
		return nil
	}
	return ErrRead
}

// Gzip decompresses the source archive file to the destination directory.
// The source file is expected to be a gzip compressed file. Unlike the other
// container formats, [gzip] only compresses a single file.
//
// [gzip]: https://www.gnu.org/software/gzip/
func (x Extractor) Gzip() error {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath("gzip")
	if err != nil {
		return fmt.Errorf("archive gzip extract %w", err)
	}
	if dst == "" {
		return ErrDest
	}

	tmpFile := filepath.Join(dst, "archive.gz")
	if _, err := helper.DuplicateOW(src, tmpFile); err != nil {
		return fmt.Errorf("archive gzip duplicate %w", err)
	}

	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutExtract)
	defer cancel()
	const (
		decompress = "--decompress" // -d decompress
		restore    = "--name"       // -n restore original name and timestamp
		overwrite  = "--force"      // -f overwrite existing files
	)
	args := []string{decompress, restore, overwrite, tmpFile}
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return fmt.Errorf("archive gzip %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("archive gzip %w: %s", err, prog)
	}
	return nil
}
