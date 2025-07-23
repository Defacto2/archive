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
	zipfile, err := os.OpenFile(dest, createUnique, helper.WriteWriteRead)
	if err != nil {
		return 0, fmt.Errorf("rezip compress failed to open file: %w", err)
	}
	defer zipfile.Close()

	deflater := zip.NewWriter(zipfile)
	defer deflater.Close()

	dst, err := deflater.Create(filepath.Base(name))
	if err != nil {
		return 0, fmt.Errorf("rezip compress failed to create writer: %w", err)
	}
	src, err := os.Open(name)
	if err != nil {
		return 0, fmt.Errorf("rezip compress failed to open file: %w", err)
	}
	defer src.Close()

	const size = 64 * 1024
	buf := make([]byte, size)
	n, err := io.CopyBuffer(dst, src, buf)
	if err != nil {
		return 0, fmt.Errorf("rezip compress failed to copy file: %w", err)
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
	zipfile, err := os.OpenFile(dest, createUnique, helper.WriteWriteRead)
	if err != nil {
		return 0, fmt.Errorf("rezip compress dir failed to open file: %w", err)
	}
	defer zipfile.Close()

	deflater := zip.NewWriter(zipfile)
	defer deflater.Close()

	var written int64
	addFile := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("add file: %w", err)
		}
		if info.IsDir() {
			return nil
		}
		if self := path == root; self {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("add file: %w", err)
		}
		dst, err := deflater.Create(rel)
		if err != nil {
			return fmt.Errorf("add file: %w", err)
		}
		src, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("add file: %w", err)
		}
		defer src.Close()

		const size = 64 * 1024
		buf := make([]byte, size)
		n, err := io.CopyBuffer(dst, src, buf)
		if err != nil {
			return fmt.Errorf("add file: %w", err)
		}
		written += n
		return nil
	}

	err = filepath.Walk(root, addFile)
	if err != nil {
		return 0, fmt.Errorf("rezip compress dir failed to add file: %w", err)
	}

	return written, nil
}

// Test runs the rezip test command on the named file. If the file is a directory
// or empty, an error is returned. If the test command fails, an error is returned.
func Test(name string) error {
	path, err := exec.LookPath(command.Unzip)
	if err != nil {
		return fmt.Errorf("rezip test failed to find rezip executable: %w", err)
	}
	inf, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf("rezip test failed to stat file: %w", err)
	}
	if inf.IsDir() {
		return fmt.Errorf("%w: %s is a directory", ErrTest, name)
	}
	if inf.Size() == 0 {
		return fmt.Errorf("%w: %s is empty", ErrTest, name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), command.TimeoutList)
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
