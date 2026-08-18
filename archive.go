// Package archive provides compressed and stored archive file extraction and content listing.
//
// The file archive formats supported are 7-Zip, ARC, ARJ, CAB, LHA, LZH, RAR, TAR, compressed TAR, and ZIP.
//
// ZIP includes the deflate, implode, shrink, and store methods.
//
// The package uses following Linux terminal programs for legacy file support.
//
//  1. [7zz] - 7-Zip for Linux: console version
//  2. [arc] - arc - pc archive utility
//  2. [arj] - "Open-source ARJ" v3.10 (but not functional on modern macOS for Apple Silicon)
//  3. [lha] - Lhasa v0.4 LHA tool found in the jlha-utils or lhasa packages
//  4. [hwzip] - hwzip for BBS era ZIP file that uses obsolete compression methods
//  5. [tar] - GNU tar
//  6. [unrar] - 6.24 freeware by Alexander Roshal, not the common [unrar-free] which is feature incomplete
//  7. [zipinfo] - ZipInfo v3 by the Info-ZIP workgroup
//  8. [gcab] - Found with in Linux is in the Gnome msitools package
//
// [7zz]: https://www.7-zip.org/
// [arc]: https://linux.die.net/man/1/arc
// [arj]: https://arj.sourceforge.net/
// [lha]: https://fragglet.github.io/lhasa/
// [hwzip]: https://www.hanshq.net/zip.html
// [tar]: https://www.gnu.org/software/tar/
// [unrar]: https://www.rarlab.com/rar_add.htm
// [unrar-free]: https://gitlab.com/bgermann/unrar-free
// [zipinfo]: https://infozip.sourceforge.net/
// [gcab]: https://man.archlinux.org/man/gcab.1.en
package archive

// More details on Linux decompression programs:
// https://wiki.archlinux.org/title/Archiving_and_compression

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/Defacto2/archive/sanitize"
	"github.com/Defacto2/helper"
	"github.com/Defacto2/magicnumber"
)

const (
	// WriteWriteRead is the file mode for read and write access.
	// The file owner and group has read and write access, and others have read access.
	WriteWriteRead   fs.FileMode = 0o664
	DirWriteReadRead fs.FileMode = 0o755
	WriteOnly                    = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	WriteRead                    = os.O_CREATE | os.O_RDWR | os.O_TRUNC
)

const (
	arcx  = ".arc" // ARC by System Enhancement Associates
	arjx  = ".arj" // Archived by Robert Jung
	bz2x  = ".bz2" // Bzip2 by Julian Seward
	cabx  = ".cab" // Microsoft Cabinet by Microsoft
	gzipx = ".gz"  // GNU Zip by Jean-loup Gailly and Mark Adler
	lhax  = ".lha" // LHarc by Haruyasu Yoshizaki (Yoshi)
	lhzx  = ".lzh" // LHArc by Haruyasu Yoshizaki (Yoshi)
	pakx  = ".pak" // PAK by NoGate Associates
	rarx  = ".rar" // Roshal ARchive by Alexander Roshal
	tarx  = ".tar" // Tape ARchive by AT&T Bell Labs
	tgzx  = ".tgz" // Tape ARchive by AT&T Bell Labs and GNU Zip
	xzx   = ".xz"  // XZ Utils
	zipx  = ".zip" // Phil Katz's ZIP for MS-DOS systems
	zip7x = ".7z"  // 7-Zip by Igor Pavlov
	zstdx = ".zst" // Zstandard by Yann Collet
)

var (
	ErrCorruption     = errors.New("corrupt or unexpected decompression")
	ErrDest           = errors.New("destination is empty")
	ErrHLExt          = errors.New("not a valid extension, it must be in the format, .ext")
	ErrNotArchive     = errors.New("file is not an archive")
	ErrNotImplemented = errors.New("archive format is not implemented")
	ErrRead           = errors.New("could not read the file archive")
	ErrProg           = errors.New("program error")
	ErrFile           = errors.New("path is a directory")
	ErrPath           = errors.New("path is a file")
	ErrSize           = errors.New("size cannot be a negative value")
	ErrTooMany        = errors.New("will not decompress this archive as it is very large")
)

type handler int

const (
	handleNone handler = iota
	handleAppleSilicon
	handle7zip
	handleArc
	handleArj
	handleBSDTar
	handleBz2
	handleCab
	handleGzip
	handleLha
	handleRar
	handleTar
	handleTarballGz
	handleTarballBz2
	handleTarballXz
	handleTarballZst
	handleUnar
	handleZipHW
	handleZips
	handleZStandard
)

func handles(sign magicnumber.Signature, filename string) handler { //nolint:cyclop
	// TODO: future logic:
	// - attempt sign first
	// - if unknown, attempt with file extension
	// - finally return an error?
	// - need special cases with compressed tarballs that rely on filename ext
	if ok := handleMacOS(sign); ok {
		return handleAppleSilicon
	}
	// handle known tarballs first, otherwise they won't be fully decompressed
	if tarball := handleTarball(sign, filename); tarball != handleNone {
		return tarball
	}

	switch sign { //nolint:exhaustive
	case magicnumber.X7zCompressArchive:
		return handle7zip
	case magicnumber.ARChiveSEA:
		return handleArc
	case magicnumber.ArchiveRobertJung:
		return handleArj
	case magicnumber.XZCompressArchive:
		return handleBSDTar
	case magicnumber.Bzip2CompressArchive:
		return handleBz2
	case magicnumber.MicrosoftCABinet:
		return handleCab
	case magicnumber.GzipCompressArchive:
		return handleGzip
	case magicnumber.YoshiLHA:
		return handleLha
	case
		magicnumber.RoshalARchive,
		magicnumber.RoshalARchivev5:
		return handleRar
	case magicnumber.TapeARchive:
		return handleTar
	case magicnumber.NoGatePAK:
		return handleUnar
	case
		magicnumber.PKWAREZipReduce,
		magicnumber.PKWAREZipShrink:
		return handleZipHW // TODO: replace and test with new packages
	case
		magicnumber.PKWAREZip,
		magicnumber.PKWAREZip64,
		magicnumber.PKWAREZipImplode:
		return handleZips
	case magicnumber.ZStandardArchive:
		return handleZStandard
	default: // none is kept as an unused value
		return handleNone
	}
}

// handleMacOS returns true for apple silicon devices that require
// fallback utilities. Apple is locking down the operating system
// and retiring old, 32-bit era unix libraries compiled on x86,
// that are needed for some terminal tools.
func handleMacOS(sign magicnumber.Signature) bool {
	if !AccessViolation() {
		return false
	}
	switch sign { //nolint:exhaustive
	case
		magicnumber.ArchiveRobertJung,
		magicnumber.RoshalARchive,
		magicnumber.RoshalARchivev5:
		return true
	default:
		return false
	}
}

func handleTarball(sign magicnumber.Signature, filename string) handler { //nolint:cyclop
	const (
		bz2  = ".tar.bz2"
		bz   = ".tar.bz"
		gz   = ".tar.gz"
		xz   = ".tar.xz"
		zst  = ".tar.zst"
		tbz2 = ".tbz2"
		tbz  = ".tbz"
		tgz  = ".tgz"
		txz  = ".txz"
		tzst = ".tzst"
	)
	compounds := [10]string{bz2, bz, tbz2, tbz, gz, tgz, xz, txz, zst, tzst}
	compound := func(filename string) string {
		s := strings.ToLower(filename)
		for _, ext := range compounds {
			if strings.HasSuffix(s, ext) {
				return ext
			}
		}
		return ""
	}
	ext := compound(filename)
	if ext == "" {
		return handleNone
	}
	switch sign { //nolint:exhaustive
	case magicnumber.Bzip2CompressArchive:
		if ext == tbz2 || ext == bz2 {
			return handleTarballBz2
		}
	case magicnumber.GzipCompressArchive:
		if ext == tgz || ext == gz {
			return handleTarballGz
		}
	case magicnumber.XZCompressArchive:
		if ext == txz || ext == xz {
			return handleTarballXz
		}
	case magicnumber.ZStandardArchive:
		if ext == tzst || ext == zst {
			return handleTarballZst
		}
	}
	return handleNone
}

func handleUnknown(sign magicnumber.Signature) error {
	const format = "%w, %s"
	switch sign { //nolint:exhaustive
	case magicnumber.Unknown:
		return fmt.Errorf(format, ErrNotArchive, sign)
	default:
		return fmt.Errorf(format, ErrNotImplemented, sign)
	}
}

// HardLink is used to create a hard link to the source file
// when the filename does not have the required file extension.
//
// This is a workaround for archive programs such as arj which demands the file extension
// but when the source filename does not have one. The hardlink needs to be removed
// after usage.
//
// Returns:
//   - The absolute path of the hardlink is returned if it is created.
//   - An empty string is returned if the source file already has the file extension.
//   - An error is returned if the source file cannot be linked.
func HardLink(require, src string) (string, error) {
	const format = "hardlink "
	if filepath.Ext(require) == "" {
		return "", fmt.Errorf(format+"require %w '%s'", ErrHLExt, require)
	}
	if strings.EqualFold(filepath.Ext(src), require) {
		return "", nil
	}

	oldpath, err := filepath.Abs(src)
	if err != nil {
		return "", fmt.Errorf(format+"filepath abs: %w", err)
	}

	dir := filepath.Dir(oldpath)
	base := filepath.Base(oldpath)
	pattern := fmt.Sprintf("%s-*%s", base, require)

	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf(format+"create temp: %w", err)
	}
	newpath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(newpath)
		return "", fmt.Errorf(format+"close temp: %w", err)
	}
	if err := os.Remove(newpath); err != nil {
		return "", fmt.Errorf(format+"remove placeholder: %w", err)
	}

	if err := os.Link(oldpath, newpath); err != nil {
		if _, cpErr := helper.Duplicate(oldpath, newpath); cpErr != nil {
			return "", fmt.Errorf(format+"os link (%w) and duplicate: %w", err, cpErr)
		}
	}

	return newpath, nil
}

// AccessViolation returns true when the host runs macOS on Apple Silicon.
//
// runtime GOOS is "darwin" and GOARCH is "arm64".
func AccessViolation() bool {
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

// ExtractAll extracts all files from the src archive file to the destination directory.
func ExtractAll(ctx context.Context, src, dst string) error {
	const format = "extract all: %w"
	e := Extractor{Source: src, Destination: dst}
	if err := e.Extract(ctx); err != nil {
		return fmt.Errorf(format, err)
	}
	return nil
}

// ExtractSource extracts the source file to a temporary directory,
// to function as a pseudo cache.
// The named file is used as part of the extracted directory path.
// The src is the source file to extract.
//
// If a target temporary directory already exists,
// and it contains two or more entries (directories, files, etc).
// Then ExtractSource will stop, assuming the src file has already
// been extracted.
//
// If the source archive is larger then 157_286_400 bytes,
// then an error is returned.
//
// The returned abs string is the absolute path to the temporary directory
// holding the extracted archive.
func ExtractSource(ctx context.Context, src, name string) ( //nolint:cyclop,funlen
	abs string, err error,
) {
	const format = "extract source archive %s %w"

	const mb150 = 150 * 1024 * 1024

	if inf, err := os.Stat(src); err != nil {
		return "", fmt.Errorf(format, "stat source", err)
	} else if inf.IsDir() {
		return "", ErrNotArchive
	} else if inf.Size() > mb150 {
		return "", ErrTooMany
	}

	file, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf(format, "open", err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil {
			err = errors.Join(err, fmt.Errorf(format, "cannot close", cErr))
		}
	}()

	sign, err := magicnumber.Archive(file)
	if err != nil {
		return "", fmt.Errorf(format, "magic", err)
	}

	local := sanitize.Name(src)
	dst, err := helper.MkContent(local)
	if err != nil {
		return "", fmt.Errorf(format, "content directory", err)
	}

	// clean temporary extraction directory, but only if there is an error
	defer func() {
		if err != nil {
			if cErr := os.RemoveAll(dst); cErr != nil {
				err = errors.Join(err, fmt.Errorf(format, "cleanup", cErr))
			}
		}
	}()

	if sign == magicnumber.Unknown {
		// handle non-archive files
		newpath := filepath.Join(dst, name)
		if _, cErr := helper.DuplicateOW(src, newpath); cErr != nil {
			return "", fmt.Errorf(format, "duplicate file", cErr)
		}
		return dst, nil
		// return "", fmt.Errorf(format, "unknown", ErrNotArchive)
	}

	entries := 0
	// counter is used instead of os.ReadDir to handle edge-case
	// archives that contain a single directory in the root.
	counter := func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			entries++
		}
		return nil
	}
	_ = filepath.WalkDir(dst, counter)

	const extracted = 2
	if entries >= extracted {
		return dst, nil
	}

	x := Extractor{Source: src, Destination: dst}
	if err := x.Extract(ctx); err != nil {
		return "", fmt.Errorf(format, "exec", err)
	}
	return dst, nil
}

// List returns the files within a 7zip, arc, arj, lha/lhz, gzip, rar, tar, zip archive.
// The filename extension is used to determine the archive format.
func List(ctx context.Context, src, filename string) ([]string, error) {
	const format = "archive list %s %w"
	base := filepath.Base(src)
	inf, err := os.Stat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf(format, base, fs.ErrNotExist)
	}
	if inf == nil {
		return nil, nil
	}
	if inf.IsDir() {
		return nil, fmt.Errorf(format, base, ErrFile)
	}

	path, err := ExtractSource(ctx, src, filename)
	if err != nil {
		return commander(ctx, src, filename)
	}
	defer os.RemoveAll(path)

	var files []string
	err = filepath.WalkDir(path, func(targPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, relPath(path, targPath))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf(format, path, err)
	}

	return files, nil
}

func relPath(path, targPath string) string {
	rel, err := filepath.Rel(path, targPath)
	if err != nil {
		return targPath
	}
	return rel
}

// commander uses system archiver and decompression programs to read the src archive file.
func commander(ctx context.Context, src, filename string) ([]string, error) {
	cont := Content{
		Ext:   "",
		Files: []string{},
	}
	const format = "commander failed with %s (ext: %s): %w"
	if err := cont.Read(ctx, src); err != nil {
		return nil, fmt.Errorf(format, filename, cont.Ext, err)
	}

	// remove empty entries in-place
	files := slices.DeleteFunc(cont.Files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})

	return files, nil
}

func skipName(name string, targets ...string) bool {
	if len(targets) == 0 {
		return false
	}
	if !slices.Contains(targets, name) {
		return true
	}
	return false
}
