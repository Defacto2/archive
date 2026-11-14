package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const TestLbr1 = "TEST.LBR"

func TestLbrContent(t *testing.T) {
	t.Parallel()

	src := filepath.Join(testdata, TestLbr1)
	var c archive.Content
	err := c.LBR(t.Context(), src)
	be.Err(t, err, nil)

	const want = 3
	be.Equal(t, c.Ext, ".lbr")
	be.Equal(t, len(c.Files), want)
	testingTexts(t, c.Files...)
}

func TestLbrExtractor(t *testing.T) {
	t.Parallel()

	src := filepath.Join(testdata, TestLbr1)
	x := archive.Extractor{
		Source:      src,
		Destination: t.TempDir(),
	}
	err := x.LBR(t.Context())
	be.Err(t, err, nil)
	testingXTexts(t, x.Destination)
}

func TestLbrCombo(t *testing.T) {
	t.Parallel()

	src := filepath.Join(testdata, TestLbr1)
	files, err := archive.Lists(t.Context(), src)
	be.Err(t, err, nil)
	be.Equal(t, len(files), 3)
	testingTexts(t, files...)

	dst := t.TempDir()
	err = archive.ExtractAll(t.Context(), src, dst)
	be.Err(t, err, nil)
	testingXTexts(t, dst)
}
