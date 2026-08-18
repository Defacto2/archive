package archive

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/Defacto2/archive/command"
)

// BSDTar returns the content of the archive using the [bsdtar program].
//
// [bsdtar program]: https://man.freebsd.org/cgi/man.cgi?query=bsdtar&sektion=1&format=html
func (c *Content) BSDTar(ctx context.Context, src string) error {
	const format = "content tar %s %w"
	const file = command.BSDTar
	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const list = "-t"
	const location = "-f"
	out, err := c.Run(ctx, file, prog, list, location, src)
	if err != nil {
		return err
	}

	if len(out) == 0 {
		return ErrRead
	}

	c.Files = bsdTars(out)
	c.Ext = tarx
	return nil
}

// bsdTars splits raw tar -tf output into clean, normalized filenames.
func bsdTars(out []byte) []string {
	files := strings.Split(string(out), "\n")
	files = slices.DeleteFunc(files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	for i, f := range files {
		files[i] = strings.TrimRight(f, "\r")
	}
	return files
}

// BSDTar extracts the content of the Tar archive using the [bsdtar program].
// If the targets are empty then all files are extracted.
//
// bsdtar uses the performant [libarchive library] for archive extraction:
//
// gzip, bzip2, compress, xz, lzip, lzma, tar, iso9660, zip, ar, xar,
// lha/lzh, rar, rar v5, Microsoft Cabinet, 7-zip.
//
// [bsdtar program]: https://man.freebsd.org/cgi/man.cgi?query=bsdtar&sektion=1&format=html
// [libarchive library]: http://www.libarchive.org/
func (x Extractor) BSDTar(ctx context.Context, targets ...string) error {
	const format = `extract tar %s %w`
	const file = command.BSDTar

	src, dst := x.Source, x.Destination
	if dst == "" {
		return ErrDest
	}

	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutExtract)
	defer cancel()

	// note: BSD tar uses different flags to GNU tar
	const (
		extract     = "-x"                    // -x extract files
		source      = "--file"                // --file path to archive
		targetDir   = "--cd"                  // --cd target directory
		noAcls      = "--no-acls"             // --no-acls disable ACLs
		noFlags     = "--no-fflags"           // --no-fflags disable file flags
		noModTime   = "--modification-time"   // --modification-time
		noSafeW     = "--no-safe-writes"      // --no-safe-writes
		noOwner     = "--no-same-owner"       // --no-same-owner
		noPerms     = "--no-same-permissions" // --no-same-permissions
		noXattrs    = "--no-xattrs"           // --no-xattrs disable extended attributes
		stopParsing = "--"                    // -- stop parsing switches
	)

	const size = 13
	arg := make([]string, 0, size+len(targets))
	arg = append(arg, extract, source, src, noAcls, noFlags, noSafeW, noModTime,
		noOwner, noPerms, noXattrs, targetDir, dst, stopParsing)
	arg = append(arg, targets...)

	return x.Run(ctx, file, prog, arg...)
}
