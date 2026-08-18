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

	"github.com/Defacto2/archive/command"
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
		if cErr := file.Close(); cErr != nil {
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
	if file == command.ZipInfo {
		// handle broken ZIPs that may offer a partial file listing
		if stderrBuf.Len() > 0 && len(out) > 0 {
			return out, nil
		}
	}

	stderrStr := strings.TrimSpace(stderrBuf.String())
	if stderrStr != "" {
		return nil, fmt.Errorf(format+" (%s)", "exec", file, err, stderrStr)
	}
	return nil, fmt.Errorf(format, "exec", file, err)
}

func (c *Content) read(ctx context.Context, sign magicnumber.Signature, src string) error { //nolint:cyclop
	filename := filepath.Base(src)
	switch handles(sign, filename) { //nolint:exhaustive
	case handleAppleSilicon:
		return c.Lsar(ctx, src)
	case handle7zip:
		return c.Zip7(ctx, src)
	case handleArc:
		return c.ARC(ctx, src)
	case handleArj:
		return c.ARJ(ctx, src)
	case handleBSDTar:
		return c.Lsar(ctx, src)
	case handleBz2:
		return c.Bz2(ctx, src)
	case handleCab:
		return c.Cab(ctx, src)
	case handleGzip:
		return c.Gzip(ctx, src)
	case handleLha:
		return c.LHA(ctx, src)
	case handleRar:
		return c.Rar(ctx, src)
	case handleTar:
		return c.Tar(ctx, src)
	case handleTarballBz2:
		return c.Bz2Tar(ctx, src)
	case handleTarballGz:
		return c.GzipTar(ctx, src)
	case handleTarballXz:
		return c.Lsar(ctx, src)
	case handleTarballZst:
		return c.BSDTar(ctx, src)
	case handleUnar:
		return c.Lsar(ctx, src)
	case handleZips:
		return c.Zip(ctx, src)
	case handleZStandard: // listing the content of zst files is not supported
		return handleUnknown(sign)
	default:
		return handleUnknown(sign)
	}
}
