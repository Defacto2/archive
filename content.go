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

// Content are the result of using system programs to read the file archives.
//
//	func ListARJ() {
//	    var c archive.Content
//	    ctx := context.TODO()
//	    err := c.ARJ(ctx, "archive.arj")
//	    if err != nil {
//	        fmt.Fprintf(os.Stderr, "error: %v\n", err)
//	        return
//	    }
//	    for name := range slices.Values(c.Files) {
//	        fmt.Println(name)
//	    }
//	}
type Content struct {
	Ext   string   // Ext returns file extension of the archive.
	Files []string // Files returns list of files within the archive.
}

// Read returns the content of the src file archive with the archive format
// being detected using magic bytes.
// However, tarballs (compressed tar archives) must have a proper file
// extension:
//
//   - bzip2	".tar.bz2", ".tar.bz", ".tbz2" ".tbz"
//   - Gzip	".tar.gz", ".tgz"
//   - XZ Utils	".tar.xz", ".txz"
//   - Zstandard	".tar.zst" ".tzst"
//
// Supported formats are:
//
// 7-zip, arc, arj, bzip2, Microsoft cabinet, gzip, lha, lzh, Nogate pak, rar, tar, tarball, zip, zstandard.
func (c *Content) Read(ctx context.Context, src string) error {
	const format = "content read %s %w"

	file, err := os.Open(src)
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

	return c.read(ctx, sign, src)
}

// Run executes a content list command, capturing output, stderr, and context timeouts.
func (c *Content) Run(ctx context.Context, file, prog string, arg ...string) ([]byte, error) {
	const format = "content runner %s %s %w"
	cmd := exec.CommandContext(ctx, prog, arg...)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}

	if ctx.Err() != nil {
		return nil, fmt.Errorf(format, "timeout", file, ctx.Err())
	}

	stderrStr := strings.TrimSpace(stderrBuf.String())
	if stderrStr != "" {
		return nil, fmt.Errorf(format+" (%s)", "exec", file, err, stderrStr)
	}
	return nil, fmt.Errorf(format, "exec", file, err)
}

type contentReader func(*Content, context.Context, string) error

var contentReaders = map[handler]contentReader{ //nolint:gochecknoglobals
	handleAppleSilicon: (*Content).Lsar,
	handle7zip:         (*Content).Zip7,
	handleArc:          (*Content).ARC,
	handleArj:          (*Content).ARJ,
	handleBSDTar:       (*Content).Lsar,
	handleBz2:          (*Content).Bz2,
	handleCab:          (*Content).Cab,
	handleGzip:         (*Content).Gzip,
	handleLbr:          (*Content).LBR,
	handleLha:          (*Content).LHA,
	handleRar:          (*Content).Rar,
	handleTar:          (*Content).Tar,
	handleTarballBz2:   (*Content).Bz2Tar,
	handleTarballGz:    (*Content).GzipTar,
	handleTarballXz:    (*Content).Lsar,
	handleTarballZst:   (*Content).BSDTar,
	handleUnar:         (*Content).Lsar,
	handleZips:         (*Content).Zip,
}

func (c *Content) read(ctx context.Context, sign magicnumber.Signature, src string) error {
	h := handles(sign, filepath.Base(src))
	if fn, ok := contentReaders[h]; ok {
		return fn(c, ctx, src)
	}
	return handleUnknown(sign)
}
