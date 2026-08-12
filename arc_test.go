package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	TestArc1 = "ARC601.ARC"
)

func TestArcContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestArc1, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.ARC(t.Context(), src)
			if tt.wantErr {
				be.Err(t, err)
				return
			}

			be.Err(t, err, nil)

			count := len(c.Files)
			const want = 3
			be.Equal(t, c.Ext, ".arc")
			be.Equal(t, count, want)
			testingTexts(t, c.Files...)
		})
	}
}

func TestArcExtractor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestArc1, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			x := archive.Extractor{
				Source:      src,
				Destination: t.TempDir(),
			}
			err := x.ARC(t.Context())
			if tt.wantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			testingXTexts(t, x.Destination)
		})
	}
}
