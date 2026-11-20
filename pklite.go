package archive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/Defacto2/archive/pklite"
	"github.com/Defacto2/magicnumber"
)

// Package file pklite.go contains the PKLITE compressed MS-DOS executable methods.
//
// PKLITE is a packer for a single executable, so the "archive" holds one file
// that has the same name as the source.

const (
	comx = ".com" // MS-DOS command executable
	exex = ".exe" // MS-DOS executable
)

// maxPklite is the number of bytes read from a file to detect a PKLITE compressed COM.
const maxPklite = 1 << 16

// detect returns the archive signature of the file.
// It extends [magicnumber.Archive] with the detection of PKLITE compressed COM and EXE files,
// which do not use the PKLITE signature that is found in the EXE header.
func detect(file io.ReaderAt) (magicnumber.Signature, error) {
	sign, err := magicnumber.Archive(file)
	if err != nil || sign != magicnumber.Unknown {
		return sign, err //nolint:wrapcheck
	}
	data, err := io.ReadAll(io.NewSectionReader(file, 0, maxPklite))
	if err != nil {
		return sign, nil //nolint:nilerr
	}
	if pklite.IsCOM(data) {
		return magicnumber.PKLITE, nil
	}
	return sign, nil
}

// unpack reads and decompresses the PKLITE file.
func unpack(src string) ([]byte, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, fmt.Errorf("unpack: %w", err)
	}
	out, err := pklite.Decompress(data)
	if err != nil {
		// packed EXE files are detected but not yet supported.
		return nil, fmt.Errorf("%w, %w", ErrNotImplemented, err)
	}
	return out, nil
}

// Pklite returns the content of the src PKLITE compressed COM file.
// The only file listed is the decompressed executable.
func (c *Content) Pklite(_ context.Context, src string) error {
	out, err := unpack(src)
	if err != nil {
		return fmt.Errorf("content pklite: %w", err)
	}
	c.Files = []string{filepath.Base(src)}
	c.Ext = comx
	if bytes.HasPrefix(out, []byte("MZ")) {
		c.Ext = exex
	}
	return nil
}

// Pklite decompresses the PKLITE compressed COM file to the destination directory.
// The decompressed file is given the same name as the source.
// If the targets are not empty, then the file is only extracted when named in the targets.
func (x Extractor) Pklite(_ context.Context, targets ...string) error {
	name := filepath.Base(x.Source)
	if len(targets) > 0 && !slices.Contains(targets, name) {
		return nil
	}
	out, err := unpack(x.Source)
	if err != nil {
		return fmt.Errorf("extract pklite: %w", err)
	}
	err = os.MkdirAll(x.Destination, DirWriteReadRead)
	if err != nil {
		return fmt.Errorf("extract pklite mkdir all: %w", err)
	}
	err = os.WriteFile(filepath.Join(x.Destination, name), out, WriteWriteRead)
	if err != nil {
		return fmt.Errorf("extract pklite write file: %w", err)
	}
	return nil
}
