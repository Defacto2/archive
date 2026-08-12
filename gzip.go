package archive

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Defacto2/archive/command"
	"github.com/Defacto2/helper"
)

// Package file gzip.go contains the Gzip compression methods.

// Gzip returns the uncompressed filename of the gzip archive using standard Go libraries.
func (c *Content) Gzip(ctx context.Context, src string) error {
	const format = "content gzip %s %w"

	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf(format, "open src", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf(format, "header", ErrRead)
	}
	defer gzr.Close()

	base := strings.ToLower(filepath.Base(src))
	const tgz = tgzx + gzipx
	switch {
	case
		strings.HasSuffix(base, tgzx),
		strings.HasSuffix(base, tgz):
		return c.readTarball(ctx, src)
	}

	// prefer original filename found in the gzip header
	name := gzr.Header.Name
	if name == "" {
		name = GzipName(src)
	}

	c.Files = append(c.Files, name)
	c.Ext = gzipx
	return nil
}

func (x Extractor) Gzip(ctx context.Context, targets ...string) error {
	const format = "extract gzip %s %w"

	if x.Destination == "" {
		return ErrDest
	}

	f, err := os.Open(x.Source)
	if err != nil {
		return fmt.Errorf(format, "open source", err)
	}
	defer f.Close()

	src, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf(format, "header", err)
	}
	defer src.Close()

	buf := bufio.NewReader(src)

	if isTarStream(buf) {
		return extractTarStream(ctx, buf, x.Destination, targets)
	}

	return extractSingleGzipFile(buf, x.Source, src.Header.Name, x.Destination)

	// base := strings.ToLower(filepath.Base(x.Source))
	// const tgz = tgzx + gzipx
	// switch {
	// case
	// 	strings.HasSuffix(base, tgzx),
	// 	strings.HasSuffix(base, tgz):
	// 	return extractTarStream(ctx, src, x.Destination, targets)
	// default:
	// }
	//
	// // Handle single .gz file stream
	// path := src.Header.Name
	// if path == "" {
	// 	path = GzipName(x.Source)
	// }
	//
	// const flag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	// const perm = 0o644
	// name := filepath.Join(x.Destination, filepath.Base(path))
	// dst, err := os.OpenFile(name, flag, perm)
	// if err != nil {
	// 	return fmt.Errorf(format, "create destination file", err)
	// }
	// defer dst.Close()
	//
	// if _, err := io.Copy(dst, src); err != nil {
	// 	return fmt.Errorf(format, "decompress", err)
	// }
	// return nil
}

func isTarStream(r *bufio.Reader) bool {
	const tarHeaderLen = 512
	peek, err := r.Peek(tarHeaderLen)
	if err != nil || len(peek) < 262 {
		return false
	}
	// Bytes 257..262 contain the "ustar" magic header signature
	return string(peek[257:262]) == "ustar"
}

func extractSingleGzipFile(r io.Reader, src, headerName, dst string) error {
	outName := headerName
	if outName == "" {
		outName = GzipName(src)
	}

	dstPath := filepath.Join(dst, filepath.Base(outName))
	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("extract gzip create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, r); err != nil {
		return fmt.Errorf("extract gzip copy: %w", err)
	}
	return nil
}

// Gzip returns the uncompressed filename of the [gzip] archive which is expected to be a single file.
//
// [gzip]: https://www.gnu.org/software/gzip/
func (c *Content) _Gzip(ctx context.Context, src string) error {
	const format = "content gzip %s %w"
	const file = command.Gzip
	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const test = "-t"
	_, err = c.Run(ctx, file, prog, test, src)
	if err != nil {
		return err
	}

	base := strings.ToLower(filepath.Base(src))
	const tgz = tgzx + gzipx
	switch {
	case strings.HasSuffix(base, tgzx), strings.HasSuffix(base, tgz):
		return c.readTarball(ctx, src)
	case strings.HasSuffix(base, gzipx):
		c.readname(base)
		return nil
	}

	c.readname(base)
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
func (c *Content) readTarball(ctx context.Context, src string) (err error) {
	const format = "read tarball %w"
	tmp, err := helper.MkContent(src)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	defer func() {
		if cErr := os.RemoveAll(tmp); cErr != nil {
			err = errors.Join(err, fmt.Errorf(format, cErr))
		}
	}()
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
	defer func() {
		if cErr := os.Remove(name); cErr != nil {
			err = errors.Join(err, fmt.Errorf(format, err))
		}
	}()
	return c.Tar(ctx, name)
}

// Gzip decompresses a gzip file into x.Destination using the Go standard library.
// If the archive is a .tar.gz tarball, targets filter extracted files;
// otherwise, targets are ignored and the single file is extracted.
// func (x Extractor) __Gzip(ctx context.Context, targets ...string) error {
// 	if x.Destination == "" {
// 		return ErrDest
// 	}
// 	if err := os.MkdirAll(x.Destination, 0o755); err != nil {
// 		return fmt.Errorf("extract gzip mkdir: %w", err)
// 	}
//
// 	f, err := os.Open(x.Source)
// 	if err != nil {
// 		return fmt.Errorf("extract gzip open: %w", err)
// 	}
// 	defer f.Close()
//
// 	gzr, err := gzip.NewReader(f)
// 	if err != nil {
// 		return fmt.Errorf("extract gzip header: %w", err)
// 	}
// 	defer gzr.Close()
//
// 	base := strings.ToLower(filepath.Base(x.Source))
//
// 	// Handle .tar.gz / .tgz tarball stream
// 	if strings.HasSuffix(base, ".tar.gz") || strings.HasSuffix(base, ".tgz") {
// 		return extractTarStream(ctx, gzr, x.Destination, targets)
// 	}
//
// 	// Handle single .gz file stream
// 	outName := gzr.Header.Name
// 	if outName == "" {
// 		outName = GzipName(x.Source)
// 	}
//
// 	dstPath := filepath.Join(x.Destination, filepath.Base(outName))
// 	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
// 	if err != nil {
// 		return fmt.Errorf("extract gzip create file: %w", err)
// 	}
// 	defer out.Close()
//
// 	if _, err := io.Copy(out, gzr); err != nil {
// 		return fmt.Errorf("extract gzip decompress: %w", err)
// 	}
//
// 	return nil
// }

// extractTarStream reads a tar archive directly from an io.Reader stream.
func extractTarStream(ctx context.Context, r io.Reader, dst string, targets []string) error {
	tr := tar.NewReader(r)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("extract tar read header: %w", err)
		}

		// Filter target files if target list was provided
		if len(targets) > 0 && !slices.Contains(targets, header.Name) {
			continue
		}

		// Prevent Zip Slip / Path Traversal vulnerabilities
		targetPath := filepath.Join(dst, header.Name)
		if !strings.HasPrefix(targetPath, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("extract tar mkdir: %w", err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("extract tar mkdir parent: %w", err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("extract tar create: %w", err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("extract tar copy: %w", err)
			}
			outFile.Close()
		}
	}

	return nil
}

// Gzip decompresses the source archive file to the destination directory.
// The source file is expected to be a gzip compressed file. Unlike the other
// container formats, [gzip] only compresses a single file.
//
// The targets are only used for the tarball gzip (.tar.gz) archive format,
// otherwise it is ignored.
//
// [gzip]: https://www.gnu.org/software/gzip/
// func (x Extractor) _Gzip(ctx context.Context, targets ...string) error {
// 	m, err := x.gzip(ctx)
// 	if err != nil {
// 		return err
// 	}
// 	if m.magic == tgzx {
// 		xtb, err := opentarball(ctx, m.name, x.Destination)
// 		if err != nil {
// 			return err
// 		}
// 		return xtb.TempTar(ctx, targets...)
// 	}
// 	return nil
// }

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
