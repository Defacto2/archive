package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file lha.go contains the LHA/LZH compression methods.

// LHA returns the content of the src LHA or LZH archive.
// The format credited to Haruyasu Yoshizaki (Yoshi) using the [lha program].
//
// On Linux either the jlha-utils or lhasa work.
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
	if notLHA(out) {
		return ErrRead
	}
	c.Files = lhaFiles(out)
	c.Ext = lhax
	return nil
}

// lhaFiles parses the output of the lha list command and returns the listed filenames.
func lhaFiles(out []byte) []string {

	// PERMSSN    UID  GID      SIZE  RATIO     STAMP           NAME
	// ---------- ----------- ------- ------ ------------ --------------------
	// [generic]                 2009  48.8% Feb 14 13:21 testdat1.txt
	// [generic]                  469  66.5% Feb 14 13:17 testdat2.txt
	// [generic]                81410  29.5% Feb 14 13:21 testdat3.txt
	// ---------- ----------- ------- ------ ------------ --------------------
	//  Total         3 files   83888  30.2% Feb 14 07:19

	skip1 := []byte("PERMSSN    UID  GID")
	skip2 := []byte("---------- -----------")
	const padd = len("---------- ----------- ------- ------ ------------ ")
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
		file := strings.TrimSpace(string(line[padd:]))
		if file == "" {
			continue
		}
		files = append(files, file)
	}
	return files
}

// notLHA returns true if the output is not an LHA archive.
func notLHA(output []byte) bool {
	if len(output) == 0 {
		return true
	}
	p := bytes.ReplaceAll(output, []byte("  "), []byte(""))
	return bytes.Contains(p, []byte("Total 0 files 0"))
}

// LHA extracts the targets from the source LHA/LZH archive.
// The format credited to Haruyasu Yoshizaki (Yoshi) using the [lha program].
// If the targets are empty then all files are extracted.
//
// On Linux either the jlha-utils or lhasa work.
//
// [lha program]: https://fragglet.github.io/lhasa/
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
		extract     = "e" // "Extract archive. Files are extracted to the current working directory unless the 'w' option is specified."
		ignorepaths = "i" // "Ignore paths of archived files: extract all archived files to  the  same  directory, ignoring subdirectories."
		overwrite   = "f" // "Force overwrite of existing files: do not prompt"
		quiet       = "q1"
		quieter     = "q2"
	)
	param := fmt.Sprintf("-%s%s%sw=%s", extract, overwrite, ignorepaths, dst)
	args := []string{param, src}

	// convert targets to lowercase which is a quirk in lha
	for i, s := range targets {
		targets[i] = strings.ToLower(s)
	}
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
