package archive

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Defacto2/archive/command"
)

var (
	// ErrContext Deprecated: Unused.
	ErrContext = errors.New("context cannot be nil")
	// ErrExt Deprecated: Retired.
	ErrExt = errors.New("extension is not a supported archive format")
	// ErrPanic Deprecated: Unused.
	ErrPanic = errors.New("extract panic")
	// ErrMissing Deprecated: Unused.
	ErrMissing = errors.New("path does not exist")
)

// MagicExt uses the Linux [file] program to determine the src archive file type.
// The returned string will be a file separator and extension.
//
// Note both bzip2 and gzip archives now do not return the .tar extension prefix.
// The detection of tar.gz archives requires the src filename to end with .tar.gz,
// otherwise the file will be treated as a gzip archive.
//
// Deprecated: This func is now unused, it is frozen and no new functionality will be added.
//
// [file]: https://www.darwinsys.com/file/
func MagicExt(ctx context.Context, src string) (string, error) {
	const format = "archive magic file"
	prog, err := exec.LookPath("file")
	if err != nil {
		return "", fmt.Errorf(format+" lookup %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, command.TimeoutExtract)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, "--brief", src)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(format+" command %w", err)
	}
	if len(out) == 0 {
		return "", fmt.Errorf(format+" type: %w", ErrRead)
	}
	magics := map[string]string{
		// note these are the outputs from the `file` command
		"arc archive data":                  arcx,
		"arj archive data":                  arjx,
		"bzip2 compressed data":             bz2x,
		"microsoft cabinet archive data":    cabx,
		"gzip compressed data":              gzipx,
		"pak archive data":                  pakx,
		"rar archive data":                  rarx,
		"posix tar archive":                 tarx,
		"xz compressed data":                xzx,
		"zip archive data":                  zipx,
		"7-zip archive data":                zip7x,
		"zstandard compressed data (v0.8+)": zstdx,
	}
	result := strings.Split(strings.ToLower(string(out)), ",")
	if len(result) == 0 {
		return "", ErrNotArchive
	}
	magic := strings.TrimSpace(result[0])
	if foundLHA(magic) {
		return lhax, nil
	}
	if foundTGZ(magic, src) {
		return tgzx, nil
	}
	for pattern, ext := range magics {
		if magic == pattern {
			return ext, nil
		}
	}
	return "", fmt.Errorf(format+" %w: '%s'", ErrExt, magic)
}

// foundLHA returns true if the LHA file type is matched in the magic string.
func foundLHA(magic string) bool {
	words := strings.Split(magic, " ")
	if len(words) < 1 {
		return false
	}
	const lha, lharc = "lha", "lharc"
	if words[0] == lharc {
		return true
	}
	if words[0] != lha {
		return false
	}
	const limit = 4
	if len(words) < limit {
		return false
	}
	if strings.Join(words[0:3], " ") == "lha archive data" {
		return true
	}
	if strings.Join(words[2:4], " ") == "archive data" {
		return true
	}
	return false
}

// foundTGZ returns true if a Tar archive with Gzip compression is matched in the src file.
func foundTGZ(magic, src string) bool {
	if magic != "gzip compressed data" {
		return false
	}
	name := strings.ToLower(filepath.Base(src))
	return strings.HasSuffix(name, ".tar.gz")
}
