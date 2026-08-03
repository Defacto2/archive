package archive

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Defacto2/archive/command"
	"github.com/Defacto2/helper"
)

// Package file gzip.go contains the Gzip compression methods.

// Gzip returns the uncompressed filename of the [gzip] archive which is expected to be a single file.
//
// [gzip]: https://www.gnu.org/software/gzip/
func (c *Content) Gzip(ctx context.Context, src string) error {
	const format = "content gzip %w"
	prog, err := exec.LookPath(command.Gzip)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	const test = "-t"
	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, test, src)
	var buf bytes.Buffer
	cmd.Stderr = &buf
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf(format, err)
	}
	out = bytes.TrimSpace(out)
	const match = "not in gzip format"
	if bytes.Contains(out, []byte(match)) {
		return ErrRead
	}
	if len(out) != 0 {
		return ErrRead
	}
	base := strings.ToLower(filepath.Base(src))
	switch {
	case // tar.gz case must be before the .gz case.
		strings.HasSuffix(base, tgzx),
		strings.HasSuffix(base, ".tar.gz"):
		return c.readTarball(ctx, src)
	case strings.HasSuffix(base, gzipx):
		c.readname(base)
		return nil
	}
	return nil
}

// readname appends the uncompressed filename to the Content struct.
func (c *Content) readname(src string) {
	c.Files = append(c.Files, GzipName(src))
	c.Ext = gzipx
}

// GzipName returns the uncompressed base filename of the gzip archive.
//
// For example, if the base filename is `example.txt.gz`, the uncompressed filename is `example.txt`.
func GzipName(src string) string {
	base := filepath.Base(src)
	if i := strings.LastIndex(base, "."); i > 0 {
		return base[:i]
	}
	return base
}

// readTarball extracts and reads the gzip compressed tarball archive.
//
// This is slower than other read methods as the tarball archive is
// first decompressed to a temporary directory before being read.
func (c *Content) readTarball(ctx context.Context, src string) error {
	const format = "read tarball %w"
	tmp, err := helper.MkContent(src)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	defer os.RemoveAll(tmp)
	x := Extractor{
		Source:      src,
		Destination: tmp,
	}
	if err := x.tarball(ctx); err != nil {
		return fmt.Errorf(format, err)
	}
	s := strings.TrimSuffix(filepath.Base(src), gzipx)
	name := filepath.Join(tmp, s)
	inf, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	if inf.IsDir() {
		return fmt.Errorf(format, ErrFile)
	}
	ext, err := MagicExt(ctx, name)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	if ext != tarx {
		return nil
	}
	c.Ext = tarx
	defer os.Remove(name)
	return c.Tar(ctx, name)
}

// Gzip decompresses the source archive file to the destination directory.
// The source file is expected to be a gzip compressed file. Unlike the other
// container formats, [gzip] only compresses a single file.
//
// The targets are only used for the tarball gzip (.tar.gz) archive format,
// otherwise it is ignored.
//
// [gzip]: https://www.gnu.org/software/gzip/
func (x Extractor) Gzip(ctx context.Context, targets ...string) error {
	m, err := x.gzip(ctx)
	if err != nil {
		return err
	}
	if m.magic == tgzx {
		xtb, err := opentarball(ctx, m.name, x.Destination)
		if err != nil {
			return err
		}
		return xtb.TempTar(ctx, targets...)
	}
	return nil
}

// opentarball extracts the tarball archive from the gzip compressed file.
func opentarball(ctx context.Context, name string, dest string) (Extractor, error) {
	const format = "open tarball %w"
	empty := Extractor{Source: "", Destination: ""}
	dir := filepath.Dir(name)
	tarball := filepath.Join(dir, GzipName(name))
	_, err := os.Stat(tarball)
	if err != nil {
		return empty, fmt.Errorf(format, err)
	}
	magic, err := MagicExt(ctx, tarball)
	if err != nil {
		return empty, fmt.Errorf(format, err)
	}
	if magic != tarx {
		return empty, nil
	}
	return Extractor{Source: tarball, Destination: dest}, nil
}

type method struct {
	magic string
	name  string
}

func (x Extractor) tarball(ctx context.Context, targets ...string) error {
	m, err := x.gzip(ctx)
	if err != nil {
		return err
	}
	if m.magic == tgzx {
		xtb, err := opentarball(ctx, m.name, x.Destination)
		if err != nil {
			return err
		}
		return xtb.Tar(ctx, targets...)
	}
	return nil
}

func (x Extractor) gzip(ctx context.Context) (method, error) {
	const format = "extract gzip %w"
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Gzip)
	if err != nil {
		return method{}, fmt.Errorf(format, err)
	}
	if dst == "" {
		return method{}, ErrDest
	}

	base := filepath.Base(src)
	name := filepath.Join(dst, base)
	_, err = helper.DuplicateOW(src, name)
	if err != nil {
		return method{}, fmt.Errorf(format, err)
	}
	magic, err := MagicExt(ctx, name)
	if err != nil {
		return method{}, fmt.Errorf(format, err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutExtract)
	defer cancel()
	const (
		decompress = "--decompress" // -d decompress
		restore    = "--name"       // -n restore original name and timestamp
		overwrite  = "--force"      // -f overwrite existing files
	)
	args := []string{decompress, restore, overwrite, name}
	cmd := exec.CommandContext(ctx, prog, args...)
	var buf bytes.Buffer
	cmd.Stderr = &buf
	if err = cmd.Run(); err != nil {
		if buf.String() != "" {
			return method{}, fmt.Errorf(format+": %s: %s", ErrProg, prog, strings.TrimSpace(buf.String()))
		}
		return method{}, fmt.Errorf(format+": %s", err, prog)
	}
	return method{magic, name}, nil
}
