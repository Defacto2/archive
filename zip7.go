package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file zip7.go contains the 7-Zip compression methods.

// Zip7 returns the content of the src 7-zip archive.
// The format credited to Igor Pavlov and using the [7z program].
//
// On some Linux distributions the 7z program is named 7zz.
// The legacy version of the 7z program, the p7zip package
// should not be used!
//
// [7z program]: https://7-zip.org/
func (c *Content) Zip7(src string) error {
	prog, err := exec.LookPath(command.Zip7)
	if err != nil {
		return fmt.Errorf("content 7zip reader %w", err)
	}
	const list = "l"
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLookup)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &b
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("content 7zip output %w", err)
	}
	if not7zip(out) {
		return ErrRead
	}
	c.Files = zip7files(out)
	c.Ext = zip7x
	return nil
}

// zip7files parses the output of the 7z list command and returns the listed filenames.
func zip7files(out []byte) []string {
	//    Date      Time    Attr         Size   Compressed  Name
	// ------------------- ----- ------------ ------------  ------------------------
	// 2025-02-15 00:21:10 ....A         2009        20465  TESTDAT1.TXT
	// 2025-02-15 00:17:34 ....A          469               TESTDAT2.TXT
	// 2025-02-15 00:21:02 ....A        81410               TESTDAT3.TXT
	// ------------------- ----- ------------ ------------  ------------------------
	// 2025-02-15 00:21:10              83888        20465  3 files

	const tableEnd = 2
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
		if skipped > tableEnd {
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

// not7zip returns true if the output is not a 7z archive.
// The 7zz application supports many archive formats but in this
// case we are only interested in the 7z format.
func not7zip(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	return !bytes.Contains(output, []byte("Type = 7z"))
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
		return fmt.Errorf("extractor 7z %w", err)
	}
	if dst == "" {
		return ErrDest
	}

	// as the 7z program supports many archive formats, restrict it to 7z
	if ext, err := MagicExt(src); err != nil {
		return fmt.Errorf("extractor 7z %w: %s", err, src)
	} else if ext != zip7x {
		return fmt.Errorf("extractor 7z %w: %s", ErrExt, src)
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
			return fmt.Errorf("extractor 7z %w: %s: %s", ErrProg, prog, strings.TrimSpace(b.String()))
		}
		return fmt.Errorf("extractor 7z %w: %s", err, prog)
	}
	return nil
}
