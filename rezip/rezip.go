// Package rezip provides compression for files and directories to create
// zip archives using the universal Store and Deflate compression methods.
package rezip

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Defacto2/archive/command"
	"github.com/Defacto2/archive/pkzip"
	"github.com/Defacto2/helper"
)

const (
	testArg = "-t"

	createUnique = os.O_RDWR | os.O_CREATE | os.O_EXCL
)

var ErrTest = errors.New("rezip test failed")

// Compress compresses the named file into the dest zip file using the
// Deflate method. The total number of bytes written to the zip file is returned.
//
// The dest must be a valid file path and should include the .zip extension.
// If the dest file already exists, an error is returned.
func Compress(name, dest string) (int, error) {
	const format = "rezip compress failed to "
	zipfile, err := os.OpenFile(dest, createUnique, helper.WriteWriteRead)
	if err != nil {
		return 0, fmt.Errorf(format+"open file: %w", err)
	}
	defer zipfile.Close()

	deflater := zip.NewWriter(zipfile)
	defer deflater.Close()

	dst, err := deflater.Create(filepath.Base(name))
	if err != nil {
		return 0, fmt.Errorf(format+"create writer: %w", err)
	}
	src, err := os.Open(name)
	if err != nil {
		return 0, fmt.Errorf(format+"open file: %w", err)
	}
	defer src.Close()

	const size = 64 * 1024
	buf := make([]byte, size)
	n, err := io.CopyBuffer(dst, src, buf)
	if err != nil {
		return 0, fmt.Errorf(format+"copy file: %w", err)
	}
	return int(n), nil
}

// CompressDir compresses the named root directory into the dest zip file
// using both the Deflate method. The total number
// of bytes written to the zip file is returned.
//
// The dest must be a valid file path and should include the .zip extension.
// If the dest file already exists, an error is returned.
func CompressDir(root, dest string) (int64, error) {
	const format = "rezip compress dir"
	zipfile, err := os.OpenFile(dest, createUnique, helper.WriteWriteRead)
	if err != nil {
		return 0, fmt.Errorf(format+" failed to open file: %w", err)
	}
	defer zipfile.Close()

	deflater := zip.NewWriter(zipfile)
	defer deflater.Close()

	osr, err := os.OpenRoot(root)
	if err != nil {
		return 0, fmt.Errorf(format+" failed to open root: %w", err)
	}
	defer osr.Close()

	var written int64
	addFile := func(path string, info os.FileInfo, err error) error {
		const format = "add file: %w"
		if err != nil {
			return fmt.Errorf(format, err)
		}
		if info.IsDir() {
			return nil
		}
		if self := path == root; self {
			return nil
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf(format, err)
		}
		dst, err := deflater.Create(name)
		if err != nil {
			return fmt.Errorf(format, err)
		}
		src, err := osr.Open(name)
		if err != nil {
			return fmt.Errorf(format, err)
		}
		defer src.Close()

		const size = 64 * 1024
		buf := make([]byte, size)
		n, err := io.CopyBuffer(dst, src, buf)
		if err != nil {
			return fmt.Errorf(format, err)
		}
		written += n
		return nil
	}

	err = filepath.Walk(root, addFile)
	if err != nil {
		return 0, fmt.Errorf(format+" failed to add file: %w", err)
	}

	return written, nil
}

// Test runs the rezip test command on the named file. If the file is a directory
// or empty, an error is returned. If the test command fails, an error is returned.
func Test(ctx context.Context, name string) error {
	const format = "rezip test failed"
	path, err := exec.LookPath(command.Unzip)
	if err != nil {
		return fmt.Errorf(format+" to find rezip executable: %w", err)
	}
	inf, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf(format+" to stat file: %w", err)
	}
	if inf.IsDir() {
		return fmt.Errorf(format+": %w: %s is a directory", ErrTest, name)
	}
	if inf.Size() == 0 {
		return fmt.Errorf(format+": %w: %s is empty", ErrTest, name)
	}
	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()
	err = exec.CommandContext(ctx, path, testArg, name).Run()
	if err != nil {
		diag := pkzip.ExitStatus(err)
		switch diag { //nolint:exhaustive
		case pkzip.Normal, pkzip.Warning:
			// normal or warnings are fine
			return nil
		}
		return fmt.Errorf("%w: %s", ErrTest, diag)
	}
	return nil
}
