package archive

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Defacto2/archive/command"
)

// Package file cab.go contains the Microsoft Cabinet compression methods.

// Cab returns the content of the src Cabinet archive.
// The format is credited to Microsoft.
// On Linux the format is handled with the [gcab program] by Marc-André Lureau.
//
// [gcab program]: https://man.archlinux.org/man/gcab.1.en
func (c *Content) Cab(src string) error {
	prog, err := exec.LookPath(command.Cab)
	if err != nil {
		return fmt.Errorf("cab reader %w", err)
	}
	const list = "--list"
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), command.TimeoutList)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, list, src)
	cmd.Stderr = &buf
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cab output %w", err)
	}
	if len(out) == 0 || bytes.Contains(out, []byte("The input is not of cabinet format")) {
		return ErrRead
	}
	for name := range strings.Lines(string(out)) {
		if strings.TrimSpace(name) != "" {
			c.Files = append(c.Files, name)
		}
	}
	c.Ext = cabx
	return nil
}

// Cab decompresses the source archive file to the destination directory.
// The format is credited to Microsoft.
// On Linux the format is handled with the [gcab program] by Marc-André Lureau
// which does not support targets for extraction.
//
// [gcab program]: https://man.archlinux.org/man/gcab.1.en
func (x Extractor) Cab() error {
	return x.Generic(Run{
		Program: command.Cab,
		Extract: "--extract",
	})
}
