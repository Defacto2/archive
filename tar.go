package archive

import (
	"bytes"
	"context"
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
func (c *Content) Tar(src string) error {
	const format = `content tar %w`
	prog, err := exec.LookPath(command.BSDTar)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	const list = "-tf"
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), command.TimeoutList)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &buf
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf(format, err)
	}
	if len(out) == 0 {
		return ErrRead
	}
	c.Files = strings.Split(string(out), "\n")
	c.Files = slices.DeleteFunc(c.Files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	c.Ext = tarx
	return nil
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
func (x Extractor) Tar(targets ...string) error {
	const format = `extract tar %w`
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.BSDTar)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	if dst == "" {
		return ErrDest
	}
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), command.TimeoutExtract)
	defer cancel()
	// note: BSD tar uses different flags to GNU tar
	const (
		extract     = "-x"                    // -x extract files
		source      = "--file"                // -f file path to extract
		targetDir   = "--cd"                  // -C target directory
		noAcls      = "--no-acls"             // --no-acls
		noFlags     = "--no-fflags"           // --no-fflags
		noModTime   = "--modification-time"   // --modification-time
		noSafeW     = "--no-safe-writes"      // --no-safe-writes
		noOwner     = "--no-same-owner"       // --no-same-owner
		noPerms     = "--no-same-permissions" // --no-same-permissions
		noXattrs    = "--no-xattrs"           // --no-xattrs
		tarArgsBase = 12                      // base args before targets
	)
	args := make([]string, 0, tarArgsBase+len(targets))
	args = append(args, extract, source, src, noAcls, noFlags, noSafeW, noModTime,
		noOwner, noPerms, noXattrs, targetDir, dst)
	args = append(args, targets...)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &buf
	if err = cmd.Run(); err != nil {
		if buf.String() != "" {
			return fmt.Errorf(format+": %s: %s", ErrProg, prog, strings.TrimSpace(buf.String()))
		}
		return fmt.Errorf(format+": %s", err, prog)
	}
	return nil
}

// TempTar functions like Tar but removes the source tarball after extraction.
func (x Extractor) TempTar(targets ...string) error {
	tarball := x.Source
	defer os.Remove(tarball)
	return x.Tar(targets...)
}
