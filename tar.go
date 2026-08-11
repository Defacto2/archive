package archive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file tar.go contains the BSD Tar compression methods.

// Tar returns the content of the Tar archive using the [bsdtar program].
//
// [bsdtar program]: https://man.freebsd.org/cgi/man.cgi?query=bsdtar&sektion=1&format=html
func (c *Content) Tar(ctx context.Context, src string) error {
	const format = `content tar %s %w`
	prog, err := exec.LookPath(command.BSDTar)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const list = "-t"
	const location = "-f"
	cmd := exec.CommandContext(ctx, prog, list, location, src)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("content tar %s timeout: %w", src, ctx.Err())
		}

		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return fmt.Errorf(format, "exec "+src+": "+stderrStr, err)
		}
		return fmt.Errorf(format, "exec "+src, err)
	}

	if len(out) == 0 {
		return ErrRead
	}

	c.Files = BSDTar(out)
	c.Ext = tarx
	return nil
}

// BSDTar splits raw tar -tf output into clean, normalized filenames.
func BSDTar(out []byte) []string {
	files := strings.Split(string(out), "\n")
	files = slices.DeleteFunc(files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	for i, f := range files {
		files[i] = strings.TrimRight(f, "\r")
	}
	return files
}

// Tar extracts the content of the Tar archive using the [bsdtar program].
// If the targets are empty then all files are extracted.
//
// bsdtar uses the performant [libarchive library] for archive extraction:
//
// gzip, bzip2, compress, xz, lzip, lzma, tar, iso9660, zip, ar, xar,
// lha/lzh, rar, rar v5, Microsoft Cabinet, 7-zip.
//
// [bsdtar program]: https://man.freebsd.org/cgi/man.cgi?query=bsdtar&sektion=1&format=html
// [libarchive library]: http://www.libarchive.org/
func (x Extractor) Tar(ctx context.Context, targets ...string) error {
	const format = `extract tar %s %w`

	src, dst := x.Source, x.Destination
	if dst == "" {
		return ErrDest
	}

	prog, err := exec.LookPath(command.BSDTar)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutExtract)
	defer cancel()

	// note: BSD tar uses different flags to GNU tar
	const (
		extract    = "-x"                    // -x extract files
		source     = "--file"                // --file path to archive
		targetDir  = "--cd"                  // --cd target directory
		noAcls     = "--no-acls"             // --no-acls disable ACLs
		noFlags    = "--no-fflags"           // --no-fflags disable file flags
		noModTime  = "--modification-time"   // --modification-time
		noSafeW    = "--no-safe-writes"      // --no-safe-writes
		noOwner    = "--no-same-owner"       // --no-same-owner
		noPerms    = "--no-same-permissions" // --no-same-permissions
		noXattrs   = "--no-xattrs"           // --no-xattrs disable extended attributes
		stopSwitch = "--"                    // -- stop parsing switches
	)

	const size = 13
	arg := make([]string, 0, size+len(targets))
	arg = append(arg, extract, source, src, noAcls, noFlags, noSafeW, noModTime,
		noOwner, noPerms, noXattrs, targetDir, dst, stopSwitch)
	arg = append(arg, targets...)

	cmd := exec.CommandContext(ctx, prog, arg...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf(format, "timeout", ctx.Err())
		}

		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return fmt.Errorf(format, "exec: "+stderrStr, err)
		}
		return fmt.Errorf(format, "exec", err)
	}

	return nil
}

// TempTar functions like Tar but removes the source tarball after extraction.
func (x Extractor) TempTar(ctx context.Context, targets ...string) (err error) {
	tarball := x.Source
	if tarball != "" {
		defer func() {
			if cErr := os.Remove(tarball); cErr != nil && !errors.Is(cErr, os.ErrNotExist) {
				const format = "cannot remove temptar: %w"
				err = errors.Join(err, fmt.Errorf(format, cErr))
			}
		}()
	}
	return x.Tar(ctx, targets...)
}
