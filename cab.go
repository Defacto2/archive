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
func (c *Content) Cab(ctx context.Context, src string) error {
	const format = "content cab reader %s %w"
	const file = command.Cab
	prog, err := exec.LookPath(file)
	if err != nil {
		return fmt.Errorf(format, "look path", err)
	}

	ctx, cancel := context.WithTimeout(ctx, command.TimeoutList)
	defer cancel()

	const list = "--list" // list content without any file details
	out, err := c.Run(ctx, file, prog, list, src)
	if err != nil {
		return err
	}

	const match = "The input is not of cabinet format"
	if len(out) == 0 || bytes.Contains(out, []byte(match)) {
		return ErrRead
	}

	for line := range strings.Lines(string(out)) {
		if name := strings.TrimSpace(line); name != "" {
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
func (x Extractor) Cab(ctx context.Context) error {
	return x.Generic(ctx, Run{
		Program: command.Cab,
		Extract: "--extract",
	})
}
