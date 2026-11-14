package archive

import (
	"context"
	"fmt"

	"github.com/Defacto2/archive/lbr"
)

// Package file lbr.go contains the Library Utility (.LBR) archive methods.

// LBR returns the content of the src LBR archive.
func (c *Content) LBR(_ context.Context, src string) error {
	r, f, err := lbr.OpenReader(src)
	if err != nil {
		return fmt.Errorf("content lbr: %w", err)
	}
	defer f.Close()

	c.Files = make([]string, len(r.Files))
	for i, entry := range r.Files {
		c.Files[i] = entry.Name
	}
	c.Ext = lbrx
	return nil
}

// LBR extracts the content of the LBR archive.
// If the targets are empty then all files are extracted.
func (x Extractor) LBR(_ context.Context, targets ...string) error {
	r, f, err := lbr.OpenReader(x.Source)
	if err != nil {
		return fmt.Errorf("extract lbr: %w", err)
	}
	defer f.Close()

	err = r.ExtractAll(x.Destination, targets...)
	if err != nil {
		return fmt.Errorf("extract lbr: %w", err)
	}
	return nil
}
