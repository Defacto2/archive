//nolint:gochecknoglobals
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
	const format = "extractor extract %s: %w"
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

	lErr := x.lookup(ctx, sign, targets...)
	if lErr != nil {
		return fmt.Errorf(format, "lookup", lErr)
	}
	return nil
}

// Run executes an extraction command, capturing stderr and context timeouts.
func (x Extractor) Run(ctx context.Context, file, prog string, arg ...string) error {
	cmd := exec.CommandContext(ctx, prog, arg...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err == nil {
		return nil
	}
	const format = "run %s extractor %s: %w"
	if ctx.Err() != nil {
		return fmt.Errorf(format, file, "timeout", ctx.Err())
	}

	stderrStr := strings.TrimSpace(stderr.String())
	if stderrStr != "" {
		return fmt.Errorf(format, file, "exec with debug: "+stderrStr, err)
	}
	stdoutStr := strings.TrimSpace(stdout.String())
	if stdoutStr != "" {
		return fmt.Errorf(format, file, "exec with feedback: '"+stdoutStr+"'", err)
	}
	return fmt.Errorf(format, file, "exec without debug", err)
}

// lookup is used to determine the correct extraction method for the source archive.
//
// Compressed tarballs signatures are determined by the compression method, not the tarball format.
// For example, a file.tar.gz signature is a gzip compressed file, not a tarball.
type extractorFunc func(Extractor, context.Context, ...string) error

var extractors = map[handler]extractorFunc{
	handleAppleSilicon: Extractor.Unar,
	handle7zip:         Extractor.Zip7,
	handleArc:          Extractor.ARC,
	handleArj:          Extractor.ARJ,
	handleBSDTar:       Extractor.BSDTar,
	handleBz2:          func(x Extractor, ctx context.Context, _ ...string) error { return x.Bz2(ctx) },
	handleCab:          func(x Extractor, ctx context.Context, _ ...string) error { return x.Cab(ctx) },
	handleGzip:         Extractor.Gzip,
	handleLbr:          Extractor.LBR,
	handleLha:          Extractor.Unar,
	handleRar:          Extractor.Rar,
	handleTar:          Extractor.Tar,
	handleTarballBz2:   Extractor.BSDTar,
	handleTarballGz:    Extractor.Gzip,
	handleTarballXz:    Extractor.BSDTar,
	handleTarballZst:   Extractor.BSDTar,
	handleUnar:         Extractor.Unar,
	handleZips:         Extractor.Zip,
	handleZStandard:    Extractor.Zip7,
}

func (x Extractor) lookup(ctx context.Context, sign magicnumber.Signature, targets ...string) error {
	h := handles(sign, filepath.Base(x.Source))
	if fn, ok := extractors[h]; ok {
		return fn(x, ctx, targets...)
	}
	return handleUnknown(sign)
}
