package archive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file unar.go is usable for most compression methods.

// Lsar extracts the targets from the source of multiple archive types.
//
// More information:
//   - the unarchiver app: https://theunarchiver.com/
//   - unar command line tool: https://theunarchiver.com/command-line
func (c *Content) Lsar(src string) error {
	const format = `content lsar %w`
	prog, err := exec.LookPath(command.Lsar)
	if err != nil {
		return fmt.Errorf(format, err)
	}

	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), command.TimeoutList)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, src, "-json")
	cmd.Stderr = &buf

	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf(format, err)
	}

	c.Ext, c.Files, err = Lsar(out)
	if err != nil {
		return fmt.Errorf(format, err)
	}
	return nil
}

// LsarJSON stores selective metadata from the lsar JSON format.
type LsarJSON struct {
	Format   string `json:"lsarFormatName"`
	Contents []struct {
		FileName string `json:"XADFileName"` //nolint:tagliatelle
	} `json:"lsarContents"`
}

// Lsar parses the output from the lsar command with the "-json" flag.
//
// It returns an empty string for use with the [Content.Ext]
// and a string slice with the filenames for [Content.Files].
func Lsar(data []byte) (string, []string, error) {
	var out LsarJSON
	if err := json.Unmarshal(data, &out); err != nil {
		return "", nil, fmt.Errorf("json unmarshal: %w", err)
	}
	names := make([]string, 0, len(out.Contents))
	for _, entry := range out.Contents {
		if entry.FileName != "" {
			names = append(names, entry.FileName)
		}
	}
	const ext = ""
	return ext, names, nil
}

// Unar extracts the targets from the source of multiple archive types.
// If the targets are empty then all files are extracted.
//
// Support includes:
//   - ARJ, ARC, PAK, Zoo, LHZ, LZX, Squeeze, Crunch
//   - PackIt, RAR, CAB, MSI
//   - ISO, BIN, MDF, NRG, CDI
//
// More information:
//   - the unarchiver app: https://theunarchiver.com/
//   - unar command line tool: https://theunarchiver.com/command-line
func (x Extractor) Unar(targets ...string) error {
	const fmtext = "extract unar %w"
	src, dst := x.Source, x.Destination
	prog, err := exec.LookPath(command.Unar)
	if err != nil {
		return fmt.Errorf(fmtext, err)
	}
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), command.TimeoutDefunct)
	defer cancel()
	// example command: unar -quiet -no-directory -copy-time -force-overwrite -output-directory <destdir> archive [items]
	args := []string{"-force-overwrite", "-no-directory", "-output-directory", dst, src}
	if len(targets) > 0 {
		args = append(args, targets...)
	}
	const format = fmtext + `: %s: cmd errors: %q cmd out: %q`
	// Usage: unar [options] archive [files ...]
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stderr = &buf
	b, err := cmd.Output()
	cmdout := strings.ReplaceAll(strings.TrimSpace(string(b)), "\n", " ")
	cmderr := strings.TrimSpace(buf.String())
	if err != nil {
		const filefailOkay = "Opening file failed"
		if strings.Contains(cmdout, filefailOkay) {
			return nil
		}
		cmderr = fmt.Sprintf("%s : %s", err, cmderr)
		return fmt.Errorf(format, ErrProg, prog, cmderr, cmdout)
	}
	if len(cmdout) == 0 {
		return fmt.Errorf(format, ErrRead, prog, cmderr, cmdout)
	}
	return nil
}
