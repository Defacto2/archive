//nolint:cyclop
package archive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Defacto2/magicnumber"
)

// Extractor uses system archiver programs to extract the targets from the src file archive.
//
//	func Extract() {
//	    x := archive.Extractor{
//	        Source:      "archive.arj",
//	        Destination: os.TempDir(),
//	    }
//	    err := x.Extract("README.TXT", "INFO.DOC")
//	    if err != nil {
//	        fmt.Fprintf(os.Stderr, "error: %v\n", err)
//	        return
//	    }
//	}
type Extractor struct {
	Source      string // The source archive file.
	Destination string // The extraction destination directory.
}

// Extract the targets from the source file archive
// to the destination directory a system archive program.
// If the targets are empty then all files are extracted.
//
// Some archive formats do not support targets and will always extract the whole archive:
//   - Bzip2
//   - Microsoft CAB
func (x Extractor) Extract(ctx context.Context, targets ...string) (err error) {
	const format = "extractor extract %s %w"
	file, err := os.Open(x.Source)
	if err != nil {
		return fmt.Errorf(format, "open", err)
	}

	defer func() {
		cErr := file.Close()
		if cErr != nil {
			err = errors.Join(err, fmt.Errorf(format, "cannot close", cErr))
		}
	}()

	sign, err := magicnumber.Archive(file)
	if err != nil {
		return fmt.Errorf(format, "magic", err)
	}

	return x.lookup(ctx, sign, targets...)
}

// Run executes an extraction command, capturing stderr and context timeouts.
func (x Extractor) Run(ctx context.Context, file, prog string, arg ...string) error {
	cmd := exec.CommandContext(ctx, prog, arg...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err == nil {
		return nil
	}
	const format = "run %s extractor %s %w"
	if ctx.Err() != nil {
		return fmt.Errorf(format, file, "timeout", ctx.Err())
	}

	stderrStr := strings.TrimSpace(stderrBuf.String())
	if stderrStr != "" {
		return fmt.Errorf(format, file, "exec "+stderrStr, err)
	}
	return fmt.Errorf(format, file, "exec", err)
}

// lookup is used to determine the correct extraction method for the source archive.
//
// Compressed tarballs signatures are determined by the compression method, not the tarball format.
// For example, a file.tar.gz signature is a gzip compressed file, not a tarball.
func (x Extractor) lookup(ctx context.Context, sign magicnumber.Signature, targets ...string) error {
	filename := filepath.Base(x.Source)
	switch handles(sign, filename) { //nolint:exhaustive
	case handleAppleSilicon:
		return x.Unar(ctx, targets...)
	case handle7zip:
		return x.Zip7(ctx, targets...)
	case handleArc:
		return x.ARC(ctx, targets...)
	case handleArj:
		return x.ARJ(ctx, targets...)
	case handleBSDTar:
		return x.BSDTar(ctx, targets...)
	case handleBz2:
		return x.Bz2(ctx)
	case handleCab:
		return x.Cab(ctx)
	case handleGzip:
		return x.Gzip(ctx, targets...)
	case handleLha:
		return x.Unar(ctx, targets...)
	case handleRar:
		return x.Rar(ctx, targets...)
	case handleTar:
		return x.Tar(ctx, targets...)
	case handleTarballBz2:
		return x.BSDTar(ctx, targets...)
	case handleTarballGz:
		return x.Gzip(ctx, targets...)
	case handleTarballXz:
		return x.BSDTar(ctx, targets...)
	case handleTarballZst:
		return x.BSDTar(ctx, targets...)
	case handleUnar:
		return x.Unar(ctx, targets...)
	case handleZips:
		return x.Zip(ctx, targets...)
	case handleZStandard:
		return x.Zip7(ctx, targets...)
	default:
		return handleUnknown(sign)
	}
}
