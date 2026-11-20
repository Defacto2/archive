package archive_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

var pkliteCOM = filepath.Join("pklite", "testdata", "D2_V115.COM")

func TestPkliteContent(t *testing.T) {
	t.Parallel()
	var c archive.Content
	err := c.Pklite(t.Context(), pkliteCOM)
	be.Err(t, err, nil)
	be.Equal(t, c.Ext, ".com")
	be.Equal(t, c.Files, []string{"D2_V115.COM"})

	// detection by the magic number
	var r archive.Content
	be.Err(t, r.Read(t.Context(), pkliteCOM), nil)
	be.Equal(t, r.Files, []string{"D2_V115.COM"})

	// the v1.00 beta
	var b archive.Content
	be.Err(t, b.Read(t.Context(), filepath.Join("pklite", "testdata", "D2_V1B.COM")), nil)
	be.Equal(t, b.Files, []string{"D2_V1B.COM"})
}

func TestPkliteExtract(t *testing.T) {
	t.Parallel()
	x := archive.Extractor{Source: pkliteCOM, Destination: t.TempDir()}
	be.Err(t, x.Extract(t.Context()), nil)
	fi, err := os.Stat(filepath.Join(x.Destination, "D2_V115.COM"))
	be.Err(t, err, nil)
	be.Equal(t, fi.Size(), int64(4176))

	// an unmatched target extracts nothing
	y := archive.Extractor{Source: pkliteCOM, Destination: t.TempDir()}
	be.Err(t, y.Extract(t.Context(), "OTHER.COM"), nil)
	entries, err := os.ReadDir(y.Destination)
	be.Err(t, err, nil)
	be.Equal(t, len(entries), 0)
}

func TestPkliteNot(t *testing.T) {
	t.Parallel()
	var c archive.Content
	err := c.Read(t.Context(), filepath.Join("pklite", "testdata", "D2.COM"))
	be.True(t, err != nil)
}

func TestPkliteEXE(t *testing.T) {
	t.Parallel()
	src := filepath.Join("pklite", "testdata", "LOR_V115.EXE")
	var c archive.Content
	be.Err(t, c.Read(t.Context(), src), nil)
	be.Equal(t, c.Ext, ".exe")
	be.Equal(t, c.Files, []string{"LOR_V115.EXE"})

	x := archive.Extractor{Source: src, Destination: t.TempDir()}
	be.Err(t, x.Extract(t.Context()), nil)
	fi, err := os.Stat(filepath.Join(x.Destination, "LOR_V115.EXE"))
	be.Err(t, err, nil)
	be.Equal(t, fi.Size(), int64(7680))
}
