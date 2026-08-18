package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	Test7z = "7ZIP465.7Z"
)

func Test7zContent(t *testing.T) {
	t.Parallel()

	name1 := filepath.Join(testdata, Test7z)
	var c archive.Content
	err := c.Zip7(t.Context(), name1)
	be.Err(t, err, nil)

	count := len(c.Files)
	const want = 3
	be.Equal(t, c.Ext, ".7z")
	be.Equal(t, count, want)
	testingTexts(t, c.Files...)
}

func Test7zExtractor(t *testing.T) {
	t.Parallel()

	name1 := filepath.Join(testdata, Test7z)
	x := archive.Extractor{
		Source:      name1,
		Destination: t.TempDir(),
	}
	err := x.Zip7(t.Context())
	be.Err(t, err, nil)
	testingXTexts(t, x.Destination)
}
