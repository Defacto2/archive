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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Defacto2/archive/command"
	"github.com/Defacto2/archive/internal"
	"github.com/Defacto2/archive/pkzip"
	"github.com/Defacto2/helper"
	"github.com/Defacto2/magicnumber"
)

const (
	TimeoutExtract = 15 * time.Second // TimeoutExtract is the maximum time allowed for the archive extraction.
	TimeoutDefunct = 5 * time.Second  // TimeoutDefunct is the maximum time allowed for the defunct file extraction.
	TimeoutLookup  = 2 * time.Second  // TimeoutLookup is the maximum time allowed for the program list content.
)

const (
	zip7x = ".7z"  // 7-Zip by Igor Pavlov
	arcx  = ".arc" // ARC by System Enhancement Associates
	arjx  = ".arj" // Archived by Robert Jung
	lhax  = ".lha" // LHarc by Haruyasu Yoshizaki (Yoshi)
	lhzx  = ".lzh" // LHArc by Haruyasu Yoshizaki (Yoshi)
	rarx  = ".rar" // Roshal ARchive by Alexander Roshal
	tarx  = ".tar" // Tape ARchive by AT&T Bell Labs
	zipx  = ".zip" // Phil Katz's ZIP for MS-DOS systems
)

var (
	ErrDest           = errors.New("destination is empty")
	ErrExt            = errors.New("extension is not a supported archive format")
	ErrNotArchive     = errors.New("file is not an archive")
	ErrNotImplemented = errors.New("archive format is not implemented")
	ErrRead           = errors.New("could not read the file archive")
	ErrProg           = errors.New("program error")
	ErrFile           = errors.New("path is a directory")
	ErrPath           = errors.New("path is a file")
	ErrPanic          = errors.New("extract panic")
	ErrMissing        = errors.New("path does not exist")
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
		"7-zip archive data":    zip7x,
		"arc archive data":      arcx,
		"arj archive data":      arjx,
		"bzip2 compressed data": ".bz2",
		"gzip compressed data":  ".gz",
		"rar archive data":      rarx,
		"posix tar archive":     tarx,
		"zip archive data":      zipx,
	}
	s := strings.Split(strings.ToLower(string(out)), ",")
	magic := strings.TrimSpace(s[0])
	if internal.MagicLHA(magic) {
		return lhax, nil
	}
	for magic, ext := range magics {
		if strings.TrimSpace(s[0]) == magic {
			return ext, nil
		}
	}
	return "", fmt.Errorf("archive magic file %w: %q", ErrExt, magic)
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
//	    for i, f := range c.Files {
//	        fmt.Printf("%d %s\n", i+1, f)
//	    }
//	}
type Content struct {
	Ext   string   // Ext returns file extension of the archive.
	Files []string // Files returns list of files within the archive.
}

// Read returns the content of the src file archive using the system archiver programs.
// The filename is used to determine the archive format.
//
// Supported formats are ARC, ARJ, LHA, LZH, RAR, and ZIP.
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

// ARJ returns the content of the src ARJ archive,
// credited to Robert Jung, using the [arj program].
//
// [arj program]: https://arj.sourceforge.net/
func (c *Content) ARJ(src string) error {
	prog, err := exec.LookPath(command.Arj)
	if err != nil {
		return fmt.Errorf("archive arj reader %w", err)
	}
	// note: arj REQUIRES a file extension for the source archive
	srcWithExt := src
	if !strings.EqualFold(filepath.Ext(src), arjx) {
		srcWithExt = src + arjx
		if _, err := os.Stat(srcWithExt); errors.Is(err, fs.ErrNotExist) {
			newname, err := filepath.Abs(srcWithExt)
			if err != nil {
				return fmt.Errorf("archive arj symlink abs %w", err)
			}
			if err := os.Link(src, newname); err != nil {
				return fmt.Errorf("archive arj symlink %w", err)
			}
			defer os.Remove(newname)
		}
	}
	const verboselist = "l"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, verboselist, srcWithExt)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("archive arj output %w", err)
	}
	if ARJEmpty(out) {
		return ErrRead
	}
	c.Ext = arjx
	c.Files = arjfiles(out)
	return nil
}

func arjfiles(out []byte) []string {

	// Filename       Original Compressed Ratio DateTime modified Attributes/GUA BPMGS
	// ------------ ---------- ---------- ----- ----------------- -------------- -----
	// TESTDAT1.TXT       2009        889 0.443 25-02-14 13:21:10                  1
	// TESTDAT2.TXT        469        266 0.567 25-02-14 13:17:34                  1
	// TESTDAT3.TXT      81410      22438 0.276 25-02-14 13:21:02                  1
	// ------------ ---------- ---------- -----
	//      3 files      83888      23593 0.281

	skip1 := []byte("Filename       Original")
	skip2 := []byte("------------ ----------")
	files := []string{}
	skipped := 0
	for line := range bytes.Lines(out) {
		if bytes.HasPrefix(line, skip1) {
			skipped++
			continue
		}
		if bytes.HasPrefix(line, skip2) {
			skipped++
			continue
		}
		if skipped == 0 {
			continue
		}
		if skipped > 2 {
			return files
		}
		file := string(line[0:12])
		files = append(files, file)
	}
	return files
}

func ARJEmpty(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	return bytes.Contains(output, []byte("is not an ARJ archive"))
}

// LHA returns the content of the src LHA or LZH archive,
// credited to Haruyasu Yoshizaki (Yoshi), using the [lha program].
//
// The lha program on Linux does not return error codes with unsupported archive formats,
// instead a string with no results is returned.
//
//	out:  PERMSSN    UID  GID      SIZE  RATIO     STAMP           NAME
//	---------- ----------- ------- ------ ------------ --------------------
//	---------- ----------- ------- ------ ------------ --------------------
//	 Total         0 files       0 ****** Feb 14 13:44
//
// [lha program]: https://fragglet.github.io/lhasa/
func (c *Content) LHA(src string) error {
	prog, err := exec.LookPath(command.Lha)
	if err != nil {
		return fmt.Errorf("archive lha reader %w", err)
	}

	const list = "-l"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("archive lha output %w", err)
	}
	if LHAEmpty(out) {
		return ErrRead
	}

	outs := strings.Split(string(out), "\n")

	// LHA list command outputs with a MSDOS era, fixed-width layout table
	const (
		sizeS = len("[generic]              ")
		sizeL = len("-------")
		start = len("[generic]                   12 100.0% Apr 10 17:03 ")
		dir   = 0
	)

	files := []string{}
	for _, s := range outs {
		if len(s) < start {
			continue
		}
		size := strings.TrimSpace(s[sizeS : sizeS+sizeL])
		if i, err := strconv.Atoi(size); err != nil {
			continue
		} else if i == dir {
			continue
		}
		files = append(files, s[start:])
	}
	c.Files = slices.DeleteFunc(files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	c.Ext = lhax
	return nil
}

// LHAEmpty returns true if the output from an LHA or LZH list shows an empty or unsupported archive.
func LHAEmpty(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	p := bytes.ReplaceAll(output, []byte("  "), []byte(""))
	return bytes.Contains(p, []byte("Total 0 files 0"))
}

// Rar returns the content of the src RAR archive, credited to Alexander Roshal,
// using the [unrar program].
//
// [unrar program]: https://www.rarlab.com/rar_add.htm
func (c *Content) Rar(src string) error {
	prog, err := exec.LookPath(command.Unrar)
	if err != nil {
		return fmt.Errorf("archive unrar reader %w", err)
	}
	const (
		listBrief  = "lb"
		noComments = "-c-"
	)
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, listBrief, "-ep", noComments, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("archive unrar output %w: %s", err, src)
	}
	if len(out) == 0 {
		return ErrRead
	}
	c.Files = strings.Split(string(out), "\n")
	c.Files = slices.DeleteFunc(c.Files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	c.Ext = rarx
	return nil
}

func (c *Content) Zip7(src string) error {
	prog, err := exec.LookPath(command.Zip7)
	if err != nil {
		return fmt.Errorf("archive 7zip reader %w", err)
	}
	const list = "l"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("archive 7zip output %w", err)
	}
	if ARCEmpty(out) {
		return ErrRead
	}
	c.Files = zip7files(out)
	c.Ext = zip7x
	return nil
}

func zip7files(out []byte) []string {

	//    Date      Time    Attr         Size   Compressed  Name
	// ------------------- ----- ------------ ------------  ------------------------
	// 2025-02-15 00:21:10 ....A         2009        20465  TESTDAT1.TXT
	// 2025-02-15 00:17:34 ....A          469               TESTDAT2.TXT
	// 2025-02-15 00:21:02 ....A        81410               TESTDAT3.TXT
	// ------------------- ----- ------------ ------------  ------------------------
	// 2025-02-15 00:21:10              83888        20465  3 files

	skip1 := []byte("   Date      Time  ")
	skip2 := []byte("-------------------")
	const padd = len("------------------- ----- ------------ ------------  ")
	files := []string{}
	skipped := 0
	for line := range bytes.Lines(out) {
		if bytes.HasPrefix(line, skip1) {
			skipped++
			continue
		}
		if bytes.HasPrefix(line, skip2) {
			skipped++
			continue
		}
		if skipped == 0 {
			continue
		}
		if skipped > 2 {
			return files
		}
		if len(line) < padd {
			continue
		}
		file := string(line[padd:])
		files = append(files, strings.TrimSpace(file))
	}
	return files
}

// ARC returns the content of the src ARC archive, once credited to System Enhancement Associates,
// but now using the [arc port] by Howard Chu.
//
// [arc program]: https://github.com/hyc/arc
func (c *Content) ARC(src string) error {
	prog, err := exec.LookPath(command.Arc)
	if err != nil {
		return fmt.Errorf("archive arc reader %w", err)
	}
	const list = "l"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("archive arc output %w", err)
	}
	if ARCEmpty(out) {
		return ErrRead
	}
	c.Files = arcfiles(out)
	c.Ext = arcx
	return nil
}

func arcfiles(out []byte) []string {

	// Name          Length    Date
	// ============  ========  =========
	// TESTDAT1.TXT      2009  14 Feb 25
	// TESTDAT2.TXT       469  14 Feb 25
	// TESTDAT3.TXT     81410  14 Feb 25
	// 		====  ========
	// Total      3     83888

	skip1 := []byte("Name          Length    Date")
	skip2 := []byte("============  ========  =========")
	end := []byte("====  ========")
	files := []string{}
	for line := range bytes.Lines(out) {
		if bytes.HasPrefix(line, skip1) {
			continue
		}
		if bytes.HasPrefix(line, skip2) {
			continue
		}
		if bytes.HasPrefix(bytes.TrimSpace(line), end) {
			return files
		}
		file := string(line[0:12])
		files = append(files, file)
	}
	return files
}

// ARCEmpty returns true if the output from an ARC list shows an empty or unsupported archive.
func ARCEmpty(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	p := bytes.ReplaceAll(output, []byte("  "), []byte(""))
	return bytes.Contains(p, []byte("has a bad header"))
}

// Zip returns the content of the src ZIP archive, credited to Phil Katz,
// using the [zipinfo program].
//
// [zipinfo program]: https://infozip.sourceforge.net/
func (c *Content) Zip(src string) error {
	prog, err := exec.LookPath(command.ZipInfo)
	if err != nil {
		return fmt.Errorf("archive zipinfo reader %w", err)
	}
	const list = "-1"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		// handle broken zips that still contain some valid files
		if b.String() != "" && len(out) > 0 {
			// return files, zipx, nil
			return nil
		}
		// otherwise the zipinfo threw an error
		return fmt.Errorf("archive zipinfo %w: %s", err, src)
	}
	if len(out) == 0 {
		return ErrRead
	}
	c.Files = strings.Split(string(out), "\n")
	c.Files = slices.DeleteFunc(c.Files, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	c.Ext = zipx
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
	switch sign {
	case magicnumber.GzipCompressArchive:
		if err := x.Bsdtar(targets...); err != nil {
			return x.Gzip()
		}
		return nil
	case
		magicnumber.Bzip2CompressArchive,
		magicnumber.MicrosoftCABinet,
		magicnumber.TapeARchive,
		magicnumber.XZCompressArchive,
		magicnumber.ZStandardArchive:
		return x.Bsdtar(targets...)
	case
		magicnumber.PKWAREZip,
		magicnumber.PKWAREZip64,
		magicnumber.PKWAREZipShrink,
		magicnumber.PKWAREZipReduce,
		magicnumber.PKWAREZipImplode:
		return x.extractZip(targets...)
	case
		magicnumber.PKLITE,
		magicnumber.PKSFX,
		magicnumber.PKWAREMultiVolume:
		return fmt.Errorf("%w, %s", ErrNotImplemented, sign)
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
	case magicnumber.Unknown:
		return fmt.Errorf("%w, %s", ErrNotArchive, sign)
	default:
		return fmt.Errorf("%w, %s", ErrNotImplemented, sign)
	}
}

// ExtractZip delegates the extraction of the source archive to the correct program
// based on its compression method and the original operating system used to create it.
// As some valid filenames set by MS-DOS codepages are not valid UTF-8 filenames.
//
// If the ZIP file uses a passphrase an error is returned.
func (x Extractor) extractZip(targets ...string) error {
	if _, err := pkzip.Methods(x.Source); errors.Is(err, pkzip.ErrPassParse) {
		return fmt.Errorf("archive zip extract %w", err)
	}
	if err1 := x.Zip(targets...); err1 != nil {
		if err2 := x.ZipHW(targets...); err2 != nil {
			if err3 := x.Bsdtar(targets...); err3 != nil {
				return fmt.Errorf("archive zip extract %w: %w: %w", err1, err2, err3)
			}
		}
	}
	return nil
}

// Gzip decompresses the source archive file to the destination directory.
// The source file is expected to be a gzip compressed file. Unlike the other
// container formats, gzip only compresses a single file.
func (x Extractor) Gzip() error {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath("gzip")
	if err != nil {
		return fmt.Errorf("archive gzip extract %w", err)
	}
	if dst == "" {
		return ErrDest
	}

	tmpFile := filepath.Join(dst, "archive.gz")
	if _, err := helper.DuplicateOW(src, tmpFile); err != nil {
		return fmt.Errorf("archive gzip duplicate %w", err)
	}

	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutExtract)
	defer cancel()
	const (
		decompress = "--decompress" // -d decompress
		restore    = "--name"       // -n restore original name and timestamp
		overwrite  = "--force"      // -f overwrite existing files
	)
	args := []string{decompress, restore, overwrite, tmpFile}
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return fmt.Errorf("archive gzip %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("archive gzip %w: %s", err, prog)
	}
	return nil
}

// Bsdtar extracts the targets from the source archive
// to the destination directory using the [bsdtar program].
// If the targets are empty then all files are extracted.
// bsdtar uses the performant [libarchive library] for archive extraction
// and is the recommended program for extracting the following formats:
//
// gzip, bzip2, compress, xz, lzip, lzma, tar, iso9660, zip, ar, xar,
// lha/lzh, rar, rar v5, Microsoft Cabinet, 7-zip.
//
// [bsdtar program]: https://man.freebsd.org/cgi/man.cgi?query=bsdtar&sektion=1&format=html
// [libarchive library]: http://www.libarchive.org/
func (x Extractor) Bsdtar(targets ...string) error {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath("bsdtar")
	if err != nil {
		return fmt.Errorf("archive tar extract %w", err)
	}
	if dst == "" {
		return ErrDest
	}
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutExtract)
	defer cancel()
	// note: BSD tar uses different flags to GNU tar
	const (
		extract   = "-x"                    // -x extract files
		source    = "--file"                // -f file path to extract
		targetDir = "--cd"                  // -C target directory
		noAcls    = "--no-acls"             // --no-acls
		noFlags   = "--no-fflags"           // --no-fflags
		noModTime = "--modification-time"   // --modification-time
		noSafeW   = "--no-safe-writes"      // --no-safe-writes
		noOwner   = "--no-same-owner"       // --no-same-owner
		noPerms   = "--no-same-permissions" // --no-same-permissions
		noXattrs  = "--no-xattrs"           // --no-xattrs
	)
	args := []string{extract, source, src}
	args = append(args, noAcls, noFlags, noSafeW, noModTime, noOwner, noPerms, noXattrs)
	args = append(args, targetDir, dst)
	args = append(args, targets...)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return fmt.Errorf("archive tar %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("archive tar %w: %s", err, prog)
	}
	return nil
}

// ZipHW extracts the targets from the source zip archive
// to the destination directory using the [hwzip program].
// If the targets are empty then all files are extracted.
//
// [hwzip program]: https://www.hanshq.net/zip2.html
func (x Extractor) ZipHW(targets ...string) error {
	return x.Generic(Run{
		Program: command.HWZip,
		Extract: "extract",
	}, targets...)
}

// ARC extracts the targets from the source ARC archive
// to the destination directory using the [arc program].
// If the targets are empty then all files are extracted.
//
// [arc program]: https://github.com/hyc/arc
func (x Extractor) ARC(targets ...string) error {
	return x.Generic(Run{
		Program: command.Arc,
		Extract: "x",
	}, targets...)
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

// ARJ extracts the targets from the source ARJ archive
// to the destination directory using the [arj program].
// If the targets are empty then all files are extracted.
//
// [arj program]: https://arj.sourceforge.net/
func (x Extractor) ARJ(targets ...string) error {
	src, dst := x.Source, x.Destination
	if st, err := os.Stat(dst); err != nil {
		return fmt.Errorf("%w: %s", err, dst)
	} else if !st.IsDir() {
		return fmt.Errorf("%w: %s", ErrPath, dst)
	}
	// note: only use arj, as unarj offers limited functionality
	prog, err := exec.LookPath(command.Arj)
	if err != nil {
		return fmt.Errorf("archive arj extract %w", err)
	}
	// note: arj REQUIRES a file extension for the source archive
	srcWithExt := src + arjx
	if _, err := os.Stat(srcWithExt); errors.Is(err, fs.ErrNotExist) {
		if err := os.Symlink(src, srcWithExt); err != nil {
			defer os.Remove(srcWithExt)
			return fmt.Errorf("archive arj symlink %w", err)
		}
	}
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutDefunct)
	defer cancel()
	// note: these flags are for arj32 v3.10
	const (
		extract   = "x"   // x extract files
		yes       = "-y"  // -y assume yes to all queries
		targetDir = "-ht" // -ht target directory
	)
	args := []string{extract, yes, srcWithExt}
	args = append(args, targets...)
	args = append(args, targetDir+dst)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	defer os.Remove(srcWithExt)
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return fmt.Errorf("archive arj %w: %s: %q",
				ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("archive arj %w: %s", err, prog)
	}
	return nil
}

// LHA extracts the targets from the source LHA/LZH archive
// to the destination directory using an lha program.
// If the targets are empty then all files are extracted.
//
// On Linux either the jlha-utils or lhasa work.
func (x Extractor) LHA(targets ...string) error {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Lha)
	if err != nil {
		return fmt.Errorf("archive lha extract %w", err)
	}
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutDefunct)
	defer cancel()
	// example command: lha -eq2w=destdir/ archive *
	const (
		extract     = "e"
		ignorepaths = "i"
		overwrite   = "f"
		quiet       = "q1"
		quieter     = "q2"
	)
	param := fmt.Sprintf("-%s%s%sw=%s", extract, overwrite, ignorepaths, dst)
	args := []string{param, src}
	args = append(args, targets...)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		if b.String() != "" {
			return fmt.Errorf("archive lha %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("archive lha %w: %s", err, prog)
	}
	if len(out) == 0 {
		return ErrRead
	}
	return nil
}

// Rar extracts the targets from the source RAR archive
// to the destination directory using the [unrar program].
// If the targets are empty then all files are extracted.
//
// On Linux there are two versions of the unrar program, the freeware
// version by Alexander Roshal and the feature incomplete [unrar-free].
// The freeware version is the recommended program for extracting RAR archives.
//
// [unrar program]: https://www.rarlab.com/rar_add.htm
func (x Extractor) Rar(targets ...string) error {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Unrar)
	if err != nil {
		return fmt.Errorf("archive unrar extract %w", err)
	}
	if dst == "" {
		return ErrDest
	}
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutExtract)
	defer cancel()
	const (
		eXtract    = "x"   // x extract files with full path
		noPaths    = "-ep" // -ep do not preserve paths
		noComments = "-c-" // -c- do not display comments
		rename     = "-or" // -or rename files automatically
		yes        = "-y"  // -y assume yes to all queries
		outputPath = "-op" // -op output path
	)
	args := []string{eXtract, noPaths, noComments, rename, yes, src}
	args = append(args, targets...)
	args = append(args, outputPath+dst)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return fmt.Errorf("archive unrar %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("archive unrar %w: %s", err, prog)
	}
	return nil
}

// Zip extracts the targets from the source Zip archive
// to the destination directory using the [unzip program].
// If the targets are empty then all files are extracted.
//
// [unzip program]: https://www.linux.org/docs/man1/unzip.html
func (x Extractor) Zip(targets ...string) error {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Unzip)
	if err != nil {
		return fmt.Errorf("archive zip extract %w", err)
	}
	if dst == "" {
		return ErrDest
	}
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutExtract)
	defer cancel()
	// [-options]
	const (
		test            = "-t"  // test archive files
		caseinsensitive = "-C"  // use case-insensitive matching
		notimestamps    = "-DD" // skip restoration of timestamps
		junkpaths       = "-j"  // junk paths, ignore directory structures
		overwrite       = "-o"  // overwrite existing files without prompting
		quiet           = "-q"  // quiet
		quieter         = "-qq" // quieter
		targetDir       = "-d"  // target directory to extract files to
		allowCtrlChars  = "-^"  // allow control characters in filenames
	)
	// unzip [-options] file[.zip] [file(s)...] [-x files(s)] [-d exdir]
	// file[.zip]		path to the zip archive
	// [file(s)...]		optional list of archived files to process, sep by spaces.
	// [-x files(s)]	optional files to be excluded.
	// [-d exdir]		optional target directory to extract files in.
	args := []string{quieter, notimestamps, allowCtrlChars, overwrite, src}
	args = append(args, targets...)
	args = append(args, targetDir, dst)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return fmt.Errorf("archive zip %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("archive zip %w: %s", err, prog)
	}
	return nil
}

// Zip7 extracts the targets from the source 7z archive
// to the destination directory using the [7z program].
// If the targets are empty then all files are extracted.
//
// On some Linux distributions the 7z program is named 7zz.
// The legacy version of the 7z program, the p7zip package
// should not be used!
//
// [7z program]: https://www.7-zip.org/
func (x Extractor) Zip7(targets ...string) error {
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Zip7)
	if err != nil {
		return fmt.Errorf("archive 7z extract %w", err)
	}
	if dst == "" {
		return ErrDest
	}
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutExtract)
	defer cancel()
	const (
		extract   = "x"    // x extract files without paths
		overwrite = "-aoa" // -aoa overwrite all
		quiet     = "-bb0" // -bb0 quiet
		targetDir = "-o"   // -o output directory
		yes       = "-y"   // -y assume yes to all queries
	)
	args := []string{extract, overwrite, quiet, yes, targetDir + dst, src}
	args = append(args, targets...)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &b
	if err = cmd.Run(); err != nil {
		if b.String() != "" {
			return fmt.Errorf("archive 7z %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("archive 7z %w: %s", err, prog)
	}
	return nil
}
