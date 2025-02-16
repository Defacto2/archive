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
func (c *Content) Gzip(src string) error {
	prog, err := exec.LookPath(command.Gzip)
	if err != nil {
		return fmt.Errorf("content gzip %w", err)
	}
	const test = "-t"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, test, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("content gzip %w", err)
	}
	out = bytes.TrimSpace(out)
	if bytes.Contains(out, []byte("not in gzip format")) {
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
		return c.readTarball(src)
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
	s := strings.Split(base, ".")
	name := strings.Join(s[:len(s)-1], ".")
	return name
}

// readTarball extracts and reads the gzip compressed tarball archive.
//
// This is slower than other read methods as the tarball archive is
// first decompressed to a temporary directory before being read.
func (c *Content) readTarball(src string) error {
	tmp, err := helper.MkContent(src)
	if err != nil {
		return fmt.Errorf("read tarball %w", err)
	}
	defer os.RemoveAll(tmp)
	x := Extractor{
		Source:      src,
		Destination: tmp,
	}
	if err := x.tarball(); err != nil {
		return fmt.Errorf("read tarball %w", err)
	}
	s := strings.TrimSuffix(filepath.Base(src), gzipx)
	name := filepath.Join(tmp, s)
	st, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf("read tarball %w", err)
	}
	if st.IsDir() {
		return fmt.Errorf("read tarball %w", err)
	}
	ext, err := MagicExt(name)
	if err != nil {
		return fmt.Errorf("read tarball %w", err)
	}
	if ext != tarx {
		return nil
	}
	c.Ext = tarx
	defer os.Remove(name)
	return c.Tar(name)
}

// Gzip decompresses the source archive file to the destination directory.
// The source file is expected to be a gzip compressed file. Unlike the other
// container formats, [gzip] only compresses a single file.
//
// The targets are only used for the tarball gzip (.tar.gz) archive format,
// otherwise it is ignored.
//
// [gzip]: https://www.gnu.org/software/gzip/
func (x Extractor) Gzip(targets ...string) error {
	m, err := x.gzip()
	if err != nil {
		return err
	}
	if m.magic == tgzx {
		xtb, err := opentarball(m.name)
		if err != nil {
			return err
		}
		return xtb.TempTar(targets...)
	}
	return nil
}

// opentarball extracts the tarball archive from the gzip compressed file.
func opentarball(name string) (Extractor, error) {
	empty := Extractor{Source: "", Destination: ""}
	dir := filepath.Dir(name)
	tarball := filepath.Join(dir, GzipName(name))
	_, err := os.Stat(tarball)
	if err != nil {
		return empty, fmt.Errorf("open tarball %w", err)
	}
	if magic, _ := MagicExt(tarball); magic != tarx {
		return empty, nil
	}
	return Extractor{Source: tarball, Destination: dir}, nil
}

type method struct {
	magic string
	name  string
}

func (x Extractor) tarball(targets ...string) error {
	m, err := x.gzip()
	if err != nil {
		return err
	}
	if m.magic == tgzx {
		xtb, err := opentarball(m.name)
		if err != nil {
			return err
		}
		return xtb.Tar(targets...)
	}
	return nil
}

func (x Extractor) gzip() (method, error) {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Gzip)
	if err != nil {
		return method{}, fmt.Errorf("extract gzip %w", err)
	}
	if dst == "" {
		return method{}, ErrDest
	}

	base := filepath.Base(src)
	name := filepath.Join(dst, base)
	_, err = helper.DuplicateOW(src, name)
	if err != nil {
		return method{}, fmt.Errorf("extract gzip %w", err)
	}
	magic, err := MagicExt(name)
	if err != nil {
		return method{}, fmt.Errorf("extract gzip %w", err)
	}

	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutExtract)
	defer cancel()
	const (
		decompress = "--decompress" // -d decompress
		restore    = "--name"       // -n restore original name and timestamp
		overwrite  = "--force"      // -f overwrite existing files
	)
	args := []string{decompress, restore, overwrite, name}
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return method{}, fmt.Errorf("extract gzip %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return method{}, fmt.Errorf("extract gzip %w: %s", err, prog)
	}
	return method{magic, name}, nil
}
