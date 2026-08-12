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
//  2. [arj] - "Open-source ARJ" v3.10
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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/Defacto2/archive/command"
	"github.com/Defacto2/archive/pkzip"
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
	ErrContext        = errors.New("context cannot be nil")
	ErrDest           = errors.New("destination is empty")
	ErrExt            = errors.New("extension is not a supported archive format")
	ErrHLExt          = errors.New("not a valid extension, it must be in the format, .ext")
	ErrNotArchive     = errors.New("file is not an archive")
	ErrNotImplemented = errors.New("archive format is not implemented")
	ErrRead           = errors.New("could not read the file archive")
	ErrProg           = errors.New("program error")
	ErrFile           = errors.New("path is a directory")
	ErrPath           = errors.New("path is a file")
	ErrPathInsecure   = errors.New("insecure file path")
	ErrPanic          = errors.New("extract panic")
	ErrMissing        = errors.New("path does not exist")
	ErrTooMany        = errors.New("will not decompress this archive as it is very large")
)

// MagicExt uses the Linux [file] program to determine the src archive file type.
// The returned string will be a file separator and extension.
//
// Note both bzip2 and gzip archives now do not return the .tar extension prefix.
// The detection of tar.gz archives requires the src filename to end with .tar.gz,
// otherwise the file will be treated as a gzip archive.
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
		"bzip2 compressed data":             ".bz2",
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

// Content are the result of using system programs to read the file archives.
//
//	func ListARJ() {
//	    var c archive.Content
//	    err := c.ARJ("archive.arj")
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

// Read returns the content of the src file archive using the system archiver programs.
// The filename is used to determine the archive format.
//
// Supported formats are: 7-zip, arc, arj, Gzip, lha, lzh, rar, tar, zip.
func (c *Content) Read(ctx context.Context, src string) error {
	const format = "content read %w"
	ext, err := MagicExt(ctx, src)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	return c.readers(ctx, src, ext)
}

func (c *Content) readers(ctx context.Context, src, ext string) error {
	const format = "content read %w: '%s'"
	strictViolation := runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
	if strictViolation {
		switch strings.ToLower(ext) {
		case arjx, rarx:
			return c.Lsar(ctx, src)
		}
	}
	switch strings.ToLower(ext) {
	case zip7x:
		return c.Zip7(ctx, src)
	case arcx:
		return c.ARC(ctx, src)
	case arjx:
		return c.ARJ(ctx, src)
	case cabx:
		return c.Cab(ctx, src)
	case gzipx, tgzx:
		return c.Gzip(ctx, src)
	case lhax, lhzx:
		return c.LHA(ctx, src)
	case rarx:
		return c.Rar(ctx, src)
	case bz2x, tarx, xzx, zstdx:
		return c.Tar(ctx, src)
	case zipx:
		return c.Zip(ctx, src)
	case pakx:
		return c.Lsar(ctx, src)
	}
	return fmt.Errorf(format, ErrNotImplemented, ext)
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

	name := src + require

	if _, err := os.Lstat(name); err == nil {
		return name, nil
	}
	if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
		newname, err := filepath.Abs(name)
		if err != nil {
			return "", fmt.Errorf(format+"filepath abs: %w", err)
		}
		if err := os.Link(src, newname); err != nil {
			return "", fmt.Errorf(format+"os link: %w", err)
		}
		return newname, nil
	}
	return "", nil
}

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
// The required Filename string is used to determine the archive format.
//
// The following archive formats do not support targets and will always extract the whole archive.
//   - Gzip
//
// Some archive formats that could be implemented if needed in the future,
// "freearc", "zoo".
func (x Extractor) Extract(ctx context.Context, targets ...string) (err error) {
	const format = "extractor extract"
	file, err := os.Open(x.Source)
	if err != nil {
		return fmt.Errorf(format+" open %w", err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil {
			err = errors.Join(err, fmt.Errorf(format+" could not close %w", cErr))
		}
	}()
	sign, err := magicnumber.Archive(file)
	if err != nil {
		return fmt.Errorf(format+" magic %w", err)
	}
	return x.checkSign(ctx, sign, targets...)
}

// Zips attempts to delegate the extraction of the source archive to the correct
// zip decompression program on the file archive.
//
// Some filenames set by MS-DOS are not valid filenames on modern systems
// due to the use of code-points that are not valid in Unicode.
//
// If the ZIP file uses a passphrase an error is returned.
func (x Extractor) Zips(ctx context.Context, targets ...string) error {
	const format = "archive zip extract"
	_, err := pkzip.Methods(x.Source)
	if errors.Is(err, pkzip.ErrPassParse) {
		return fmt.Errorf(format+" %w", err)
	}
	err = x.Zip(ctx, targets...)
	if err == nil {
		return nil
	}
	if len(targets) > 0 {
		if err1 := x.Tar(ctx, targets...); err1 != nil {
			return fmt.Errorf(format+" all methods: %w", err)
		}
		return nil
	}
	if errhw := x.ZipHW(ctx); errhw != nil {
		if err3 := x.Tar(ctx); err3 != nil {
			return fmt.Errorf(format+" all methods: %w", err)
		}
	}
	return nil
}

// Run holds the program and extract command for use with the generic extractor.
type Run struct {
	Program string // Program is the archiver program to run, but not the full path.
	Extract string // Extract is the program command to extract files from the archive.
}

// Generic extracts the targets from the source archive
// to the destination directory using the specified archive program.
// If the targets are empty then all files are extracted.
//
// It is used for archive formats that are not widely supported
// or have a limited feature set including ARC, HWZIP, and others.
//
// These DOS era archive formats are not widely supported.
// They also does not support extracting to a target directory.
// To work around this, Generic copies the source archive
// to the destination directory, uses that as the working directory
// and extracts the files. The copied source archive is then removed.
func (x Extractor) Generic(ctx context.Context, run Run, targets ...string) (err error) {
	const format = "generic archive"
	name := run.Program
	src, dst := x.Source, x.Destination
	if inf, err := os.Stat(dst); err != nil {
		return fmt.Errorf("%w: %s", err, dst)
	} else if !inf.IsDir() {
		return fmt.Errorf("%w: %s", ErrPath, dst)
	}

	prog, err := exec.LookPath(run.Program)
	if err != nil {
		return fmt.Errorf(format+" %s extract %w", name, err)
	}

	srcInDst := filepath.Join(dst, filepath.Base(src))
	if _, err := helper.Duplicate(src, srcInDst); err != nil {
		return fmt.Errorf(format+" %s duplicate %w", name, err)
	}
	defer func() {
		if cErr := os.Remove(srcInDst); cErr != nil {
			err = errors.Join(err, fmt.Errorf(format+" %w", cErr))
		}
	}()

	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(ctx, command.TimeoutDefunct)
	defer cancel()
	const size = 2
	args := make([]string, size, size+len(targets))
	args[0] = run.Extract
	args[1] = filepath.Base(src)
	args = append(args, targets...)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Dir = dst
	cmd.Stderr = &buf
	if err = cmd.Run(); err != nil {
		if buf.String() != "" {
			return fmt.Errorf(format+" %s %w: %s: '%s'", name,
				ErrProg, prog, strings.TrimSpace(buf.String()))
		}
		return fmt.Errorf(format+" %s %w: %s", name, err, prog)
	}
	return nil
}

// checkSign is used to determine the correct extraction method for the source archive.
//
// Compressed tarballs signatures are determined by the compression method, not the tarball format.
// For example, a file.tar.gz signature is a gzip compressed file, not a tarball.
func (x Extractor) checkSign(ctx context.Context, sign magicnumber.Signature, targets ...string) error {
	if AccessViolation() {
		switch sign { //nolint:exhaustive
		case
			magicnumber.ArchiveRobertJung,
			magicnumber.RoshalARchive,
			magicnumber.RoshalARchivev5:
			return x.Unar(ctx, targets...)
		}
	}
	switch sign { //nolint:exhaustive
	case magicnumber.GzipCompressArchive:
		return x.Gzip(ctx, targets...)
	case
		magicnumber.PKWAREZipReduce,
		magicnumber.PKWAREZipShrink:
		return x.ZipHW(ctx)
	case
		magicnumber.Bzip2CompressArchive,
		magicnumber.MicrosoftCABinet,
		magicnumber.TapeARchive,
		magicnumber.XZCompressArchive,
		magicnumber.ZStandardArchive:
		return x.Tar(ctx, targets...)
	case
		magicnumber.PKWAREZip,
		magicnumber.PKWAREZip64,
		magicnumber.PKWAREZipImplode:
		return x.Zips(ctx, targets...)
	case magicnumber.ARChiveSEA:
		return x.ARC(ctx, targets...)
	case magicnumber.ArchiveRobertJung:
		return x.ARJ(ctx, targets...)
	case
		magicnumber.YoshiLHA,
		magicnumber.NoGatePAK:
		return x.Unar(ctx, targets...)
	case magicnumber.RoshalARchive,
		magicnumber.RoshalARchivev5:
		return x.Rar(ctx, targets...)
	case magicnumber.X7zCompressArchive:
		return x.Zip7(ctx, targets...)
	}
	return x.unknowns(sign)
}

func AccessViolation() bool {
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

func (x Extractor) unknowns(sign magicnumber.Signature) error {
	switch sign { //nolint:exhaustive
	case magicnumber.Unknown:
		return fmt.Errorf("%w, %s", ErrNotArchive, sign)
	default:
		return fmt.Errorf("%w, %s", ErrNotImplemented, sign)
	}
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

// ExtractSource extracts the source file into a temporary directory.
// The named file is used as part of the extracted directory path.
// The src is the source file to extract.
//
// To act as a pseudo cache, if the temporary directory already exists
// and it contains two or more items, then nothing will done and it
// is assumed the src file has already been extracted.
// This behavior can be overwritten by using [os.RemoveAll] after
// using the func.
//
// The absolute path of the extracted archive is returned.
func ExtractSource(ctx context.Context, src, name string) (abs string, err error) {
	const format = "extract source archive"
	const mb150 = 150 * 1024 * 1024
	if inf, err := os.Stat(src); err != nil {
		return "", fmt.Errorf(format+" stat file: %w", err)
	} else if inf.IsDir() {
		return "", ErrNotArchive
	} else if inf.Size() > mb150 {
		return "", ErrTooMany
	}
	dst, err := helper.MkContent(src)
	if err != nil {
		return "", fmt.Errorf(format+" cannot create content directory: %w", err)
	}
	// NOTE: os.ReadDir doesn't behave correctly with archives that
	// contain a single directory in the root, so use a custom walker.
	entries := 0
	walkerCount := func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return fs.SkipDir
		}
		if !d.IsDir() {
			entries++
		}
		return nil
	}
	_ = filepath.WalkDir(dst, walkerCount)
	const extracted = 2
	if entries >= extracted {
		return dst, nil
	}
	ok, cErr := filearchive(src)
	if cErr != nil {
		err = errors.Join(err, fmt.Errorf(format+": %w", cErr))
	}
	switch ok {
	case false:
		// copy the file
		newpath := filepath.Join(dst, name)
		if _, cErr := helper.DuplicateOW(src, newpath); cErr != nil {
			err = errors.Join(err, fmt.Errorf(format+" cannot duplicate file: %w", cErr))
		}
	case true:
		// extract the archive
		if cErr := ExtractAll(ctx, src, dst); cErr != nil {
			return "", fmt.Errorf(format+" cannot read extracted archive: %w", cErr)
		}
	}
	defer func() {
		if cErr := os.RemoveAll(dst); cErr != nil {
			err = errors.Join(err, fmt.Errorf(format+": %w", cErr))
		}
	}()
	if err != nil {
		return "", err
	}
	return dst, nil
}

// filearchive confirms whether the src file is a supported archive file.
func filearchive(src string) (ok bool, err error) {
	file, err := os.Open(src)
	if err != nil {
		return false, nil
	}
	defer func() {
		if cErr := file.Close(); err != nil {
			err = errors.Join(err, cErr)
		}
	}()
	sign, err := magicnumber.Archive(file)
	if err != nil {
		return false, nil
	}
	ok = sign != magicnumber.Unknown
	if err != nil {
		const format = "supported file archive: %w"
		return ok, fmt.Errorf(format, err)
	}
	return ok, nil
}

// List returns the files within a 7zip, arc, arj, lha/lhz, gzip, rar, tar, zip archive.
// The filename extension is used to determine the archive format.
func List(ctx context.Context, src, filename string) ([]string, error) {
	const format = "archive list %w"
	inf, err := os.Stat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf(format+": %s", ErrMissing, filepath.Base(src))
	}
	if inf == nil {
		return nil, nil
	}
	if inf.IsDir() {
		return nil, fmt.Errorf(format+": %s", ErrFile, filepath.Base(src))
	}
	path, err := ExtractSource(ctx, src, filename)
	if err != nil {
		return commander(ctx, src, filename)
	}
	var files []string
	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(path, filePath)
			if err != nil {
				fmt.Fprint(io.Discard, err)
				files = append(files, filePath)
				return nil
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf(format, err)
	}
	return files, nil
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
	// remove empty entries
	files := cont.Files
	files = slices.DeleteFunc(files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	return files, nil
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
	if ctx.Err() != nil {
		return fmt.Errorf("extract %s timeout: %w", file, ctx.Err())
	}
	stderrStr := strings.TrimSpace(stderrBuf.String())
	if stderrStr != "" {
		return fmt.Errorf("extract %s exec: %w: %s", file, err, stderrStr)
	}
	return fmt.Errorf("extract %s exec: %w", file, err)
}

// Run executes a content list command, capturing output, stderr, and context timeouts.
func (c *Content) Run(ctx context.Context, file, prog string, arg ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, prog, arg...)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}

	if ctx.Err() != nil {
		return nil, fmt.Errorf("content %s timeout: %w", file, ctx.Err())
	}

	if file == command.ZipInfo {
		// handle broken ZIPs that still returned partial file listings
		if stderrBuf.Len() > 0 && len(out) > 0 {
			return out, nil
		}
	}

	stderrStr := strings.TrimSpace(stderrBuf.String())
	if stderrStr != "" {
		return nil, fmt.Errorf("content %s exec: %w (%s)", file, err, stderrStr)
	}
	return nil, fmt.Errorf("content %s exec: %w", file, err)
}
