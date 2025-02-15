// Package archive provides compressed and stored archive file extraction and content listing.
//
// The file archive formats supported are 7Zip, ARC, ARJ, LHA, LZH, RAR, TAR, and ZIP,
// including the deflate, implode, and shrink compression methods.
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
package archive

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
	"slices"
	"strings"
	"time"

	"github.com/Defacto2/archive/pkzip"
	"github.com/Defacto2/helper"
	"github.com/Defacto2/magicnumber"
)

const (
	TimeoutExtract = 15 * time.Second // TimeoutExtract is the maximum time allowed for the archive extraction.
	TimeoutDefunct = 5 * time.Second  // TimeoutDefunct is the maximum time allowed for the defunct file extraction.
	TimeoutLookup  = 2 * time.Second  // TimeoutLookup is the maximum time allowed for the program list content.

	// WriteWriteRead is the file mode for read and write access.
	// The file owner and group has read and write access, and others have read access.
	WriteWriteRead fs.FileMode = 0o664
)

const (
	arcx  = ".arc" // ARC by System Enhancement Associates
	arjx  = ".arj" // Archived by Robert Jung
	gzipx = ".gz"  // GNU Zip by Jean-loup Gailly and Mark Adler
	lhax  = ".lha" // LHarc by Haruyasu Yoshizaki (Yoshi)
	lhzx  = ".lzh" // LHArc by Haruyasu Yoshizaki (Yoshi)
	rarx  = ".rar" // Roshal ARchive by Alexander Roshal
	tarx  = ".tar" // Tape ARchive by AT&T Bell Labs
	zipx  = ".zip" // Phil Katz's ZIP for MS-DOS systems
	zip7x = ".7z"  // 7-Zip by Igor Pavlov
)

var (
	ErrDest           = errors.New("destination is empty")
	ErrExt            = errors.New("extension is not a supported archive format")
	ErrHLExt          = errors.New("not a valid extension, it must be in the format, .ext")
	ErrNotArchive     = errors.New("file is not an archive")
	ErrNotImplemented = errors.New("archive format is not implemented")
	ErrRead           = errors.New("could not read the file archive")
	ErrProg           = errors.New("program error")
	ErrFile           = errors.New("path is a directory")
	ErrPath           = errors.New("path is a file")
	ErrPanic          = errors.New("extract panic")
	ErrMissing        = errors.New("path does not exist")
	ErrTooMany        = errors.New("will not decompress this archive as it is very large")
)

// MagicExt uses the Linux [file] program to determine the src archive file type.
// The returned string will be a file separator and extension.
//
// Note both bzip2 and gzip archives now do not return the .tar extension prefix.
//
// [file]: https://www.darwinsys.com/file/
func MagicExt(src string) (string, error) {
	prog, err := exec.LookPath("file")
	if err != nil {
		return "", fmt.Errorf("archive magic file lookup %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutExtract)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, "--brief", src)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("archive magic file command %w", err)
	}
	if len(out) == 0 {
		return "", fmt.Errorf("archive magic file type: %w", ErrRead)
	}
	magics := map[string]string{
		"arc archive data":      arcx,
		"arj archive data":      arjx,
		"bzip2 compressed data": ".bz2",
		"gzip compressed data":  gzipx,
		"rar archive data":      rarx,
		"posix tar archive":     tarx,
		"zip archive data":      zipx,
		"7-zip archive data":    zip7x,
	}
	s := strings.Split(strings.ToLower(string(out)), ",")
	magic := strings.TrimSpace(s[0])
	if foundLHA(magic) {
		return lhax, nil
	}
	for magic, ext := range magics {
		if strings.TrimSpace(s[0]) == magic {
			return ext, nil
		}
	}
	return "", fmt.Errorf("archive magic file %w: %q", ErrExt, magic)
}

// foundLHA returns true if the LHA file type is matched in the magic string.
func foundLHA(magic string) bool {
	s := strings.Split(magic, " ")
	const lha, lharc = "lha", "lharc"
	if s[0] == lharc {
		return true
	}
	if s[0] != lha {
		return false
	}
	if len(s) < len(lha) {
		return false
	}
	if strings.Join(s[0:3], " ") == "lha archive data" {
		return true
	}
	if strings.Join(s[2:4], " ") == "archive data" {
		return true
	}
	return false
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
func (c *Content) Read(src string) error {
	ext, err := MagicExt(src)
	if err != nil {
		return fmt.Errorf("read %w", err)
	}
	switch strings.ToLower(ext) {
	case zip7x:
		return c.Zip7(src)
	case arcx:
		return c.ARC(src)
	case arjx:
		return c.ARJ(src)
	case gzipx:
		return c.Gzip(src)
	case lhax, lhzx:
		return c.LHA(src)
	case rarx:
		return c.Rar(src)
	case tarx:
		return c.Tar(src)
	case zipx:
		return c.Zip(src)
	}
	return fmt.Errorf("read %w", ErrRead)
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
	if filepath.Ext(require) == "" {
		return "", fmt.Errorf("hardlink require %w %q", ErrHLExt, require)
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
			return "", fmt.Errorf("hardlink filepath abs: %w", err)
		}
		if err := os.Link(src, newname); err != nil {
			return "", fmt.Errorf("hardlink os link: %w", err)
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
// Some archive formats that could be impelmented if needed in the future,
// "freearc", "zoo".
func (x Extractor) Extract(targets ...string) error {
	r, err := os.Open(x.Source)
	if err != nil {
		return fmt.Errorf("extractor extract open %w", err)
	}
	defer r.Close()
	sign, err := magicnumber.Archive(r)
	if err != nil {
		return fmt.Errorf("extractor extract magic %w", err)
	}
	return x.checkSign(sign, targets...)
}

func (x Extractor) checkSign(sign magicnumber.Signature, targets ...string) error {
	switch sign { //nolint:exhaustive
	case magicnumber.GzipCompressArchive:
		return x.Gzip() // TODO: handle possible .tar container
	case
		magicnumber.PKWAREZipReduce,
		magicnumber.PKWAREZipShrink:
		return x.ZipHW() // TODO: work around, extract all to temp directory and move to destination
	case
		magicnumber.Bzip2CompressArchive,
		magicnumber.MicrosoftCABinet,
		magicnumber.TapeARchive,
		magicnumber.XZCompressArchive,
		magicnumber.ZStandardArchive:
		return x.Tar(targets...)
	case
		magicnumber.PKWAREZip,
		magicnumber.PKWAREZip64,
		magicnumber.PKWAREZipImplode:
		return x.Zips(targets...)
	case magicnumber.ARChiveSEA:
		return x.ARC(targets...)
	case magicnumber.ArchiveRobertJung:
		return x.ARJ(targets...)
	case magicnumber.YoshiLHA:
		return x.LHA(targets...)
	case magicnumber.RoshalARchive,
		magicnumber.RoshalARchivev5:
		return x.Rar(targets...)
	case magicnumber.X7zCompressArchive:
		return x.Zip7(targets...)
	}
	return x.unknowns(sign)
}

func (x Extractor) unknowns(sign magicnumber.Signature) error {
	switch sign { //nolint:exhaustive
	case magicnumber.Unknown:
		return fmt.Errorf("%w, %s", ErrNotArchive, sign)
	default:
		return fmt.Errorf("%w, %s", ErrNotImplemented, sign)
	}
}

// Zips attempts to delegate the extraction of the source archive to the correct
// zip decompression program on the file archive.
//
// Some filenames set by MS-DOS are not valid filenames on modern systems
// due to the use of codepoints that are not valid in Unicode.
//
// If the ZIP file uses a passphrase an error is returned.
func (x Extractor) Zips(targets ...string) error {
	if _, err := pkzip.Methods(x.Source); errors.Is(err, pkzip.ErrPassParse) {
		return fmt.Errorf("archive zip extract %w", err)
	}
	err := x.Zip(targets...)
	if err == nil {
		return nil
	}
	if len(targets) > 0 {
		if err1 := x.Tar(targets...); err1 != nil {
			return fmt.Errorf("archive zip extract all methods: %w", err)
		}
		return nil
	}
	if errhw := x.ZipHW(); errhw != nil {
		if err3 := x.Tar(); err3 != nil {
			return fmt.Errorf("archive zip extract all methods: %w", err)
		}
	}
	return nil
}

// Run is a struct that holds the program and extract command
// for use with the generic extractor.
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
func (x Extractor) Generic(run Run, targets ...string) error {
	s := run.Program
	src, dst := x.Source, x.Destination
	if st, err := os.Stat(dst); err != nil {
		return fmt.Errorf("%w: %s", err, dst)
	} else if !st.IsDir() {
		return fmt.Errorf("%w: %s", ErrPath, dst)
	}

	prog, err := exec.LookPath(run.Program)
	if err != nil {
		return fmt.Errorf("archive %s extract %w", s, err)
	}

	srcInDst := filepath.Join(dst, filepath.Base(src))
	if _, err := helper.Duplicate(src, srcInDst); err != nil {
		return fmt.Errorf("archive %s duplicate %w", s, err)
	}
	defer os.Remove(srcInDst)

	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutDefunct)
	defer cancel()
	args := []string{run.Extract, filepath.Base(src)}
	args = append(args, targets...)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Dir = dst
	cmd.Stderr = &b
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return fmt.Errorf("archive %s %w: %s: %q", s,
				ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("archive %s %w: %s", s, err, prog)
	}
	return nil
}

// ExtractAll extracts all files from the src archive file to the destination directory.
func ExtractAll(src, dst string) error {
	e := Extractor{Source: src, Destination: dst}
	if err := e.Extract(); err != nil {
		return fmt.Errorf("extract all %w", err)
	}
	return nil
}

// ExtractSource extracts the source file into a temporary directory.
// The named file is used as part of the extracted directory path.
// The src is the source file to extract.
func ExtractSource(src, name string) (string, error) {
	const mb150 = 150 * 1024 * 1024
	if st, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("cannot stat file: %w", err)
	} else if st.IsDir() {
		return "", ErrNotArchive
	} else if st.Size() > mb150 {
		return "", ErrTooMany
	}
	dst, err := helper.MkContent(src)
	if err != nil {
		return "", fmt.Errorf("cannot create content directory: %w", err)
	}
	entries, _ := os.ReadDir(dst)
	const extracted = 2
	if len(entries) >= extracted {
		return dst, nil
	}
	switch filearchive(src) {
	case false:
		// copy the file
		newpath := filepath.Join(dst, name)
		if _, err := helper.DuplicateOW(src, newpath); err != nil {
			defer os.RemoveAll(dst)
			return "", fmt.Errorf("cannot duplicate file: %w", err)
		}
	case true:
		// extract the archive
		if err := ExtractAll(src, dst); err != nil {
			defer os.RemoveAll(dst)
			return "", fmt.Errorf("cannot read extracted archive: %w", err)
		}
	}
	return dst, nil
}

// filearchive confirms if the src file is a supported archive file.
func filearchive(src string) bool {
	r, err := os.Open(src)
	if err != nil {
		return false
	}
	sign, err := magicnumber.Archive(r)
	if err != nil {
		return false
	}
	return sign != magicnumber.Unknown
}

// List returns the files within an 7zip, arc, arj, lha/lhz, gzip, rar, tar, zip archive.
// This filename extension is used to determine the archive format.
func List(src, filename string) ([]string, error) {
	st, err := os.Stat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("archive list %w: %s", ErrMissing, filepath.Base(src))
	}
	if st.IsDir() {
		return nil, fmt.Errorf("archive list %w: %s", ErrFile, filepath.Base(src))
	}
	path, err := ExtractSource(src, filename)
	if err != nil {
		return commander(src, filename)
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
		return nil, fmt.Errorf("archive list %w", err)
	}
	return files, nil
}

// commander uses system archiver and decompression programs to read the src archive file.
func commander(src, filename string) ([]string, error) {
	c := Content{
		Ext:   "",
		Files: []string{},
	}
	if err := c.Read(src); err != nil {
		return nil, fmt.Errorf("commander failed with %s (%q): %w", filename, c.Ext, err)
	}
	// remove empty entries
	files := c.Files
	files = slices.DeleteFunc(files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	return files, nil
}
