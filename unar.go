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
func (c *Content) Lsar(ctx context.Context, src string) error {
	const format = "content lsar %s %w"
	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	out, err := c.Run(ctx, src, command.Lsar, src, "-json")
	if err != nil {
		return fmt.Errorf(format, "exec", err)
	}

	c.Files, err = lsars(out)
	if err != nil {
		return fmt.Errorf(format, "parse json", err)
	}
	return nil
}

// lsarJSON stores selective metadata from the lsar JSON format.
type lsarJSON struct {
	Format   string `json:"lsarFormatName"`
	Contents []struct {
		FileName string `json:"XADFileName"` //nolint:tagliatelle
	} `json:"lsarContents"`
}

// lsars parses the output from the lsar command with the "-json" flag.
//
// It returns an empty string for use with the [Content.Ext]
// and a string slice with the filenames for [Content.Files].
func lsars(data []byte) ([]string, error) {
	var out lsarJSON
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	names := make([]string, 0, len(out.Contents))
	for _, entry := range out.Contents {
		if entry.FileName != "" {
			names = append(names, entry.FileName)
		}
	}
	return names, nil
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
func (x Extractor) Unar(ctx context.Context, targets ...string) error {
	const fmtext = "extract unar %w"
	prog, err := exec.LookPath(command.Unar)
	if err != nil {
		return fmt.Errorf(fmtext, err)
	}

	src, dst := x.Source, x.Destination
	ctx, cancel := context.WithTimeout(ctx, command.TimeoutDefunct)
	defer cancel()

	const (
		forceOverwrite  = "-force-overwrite"
		noDirectory     = "-no-directory"
		outputDirectory = "-output-directory"
	)
	const size = 5
	// example command: unar -quiet -no-directory -copy-time -force-overwrite -output-directory <destdir> archive [items]
	arg := make([]string, 0, size+len(targets))
	arg = append(arg, forceOverwrite, noDirectory, outputDirectory, dst, src)
	if len(targets) > 0 {
		arg = append(arg, targets...)
	}

	// Usage: unar [options] archive [files ...]
	cmd := exec.CommandContext(ctx, prog, arg...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	b, err := cmd.Output()

	cmdout := strings.ReplaceAll(strings.TrimSpace(string(b)), "\n", " ")
	cmderr := strings.TrimSpace(stderrBuf.String())

	const format = fmtext + `: %s: cmd errors: '%s' cmd out: '%s'`

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
