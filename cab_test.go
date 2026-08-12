package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	TestCab = "GCAB16.CAB"
)

func TestCabContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestCab, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.Cab(t.Context(), src)
			if tt.wantErr {
				be.Err(t, err)
				return
			}

			be.Err(t, err, nil)

			count := len(c.Files)
			const want = 3
			be.Equal(t, c.Ext, ".cab")
			be.Equal(t, count, want)
			testingTexts(t, c.Files...)
		})
	}
}

func TestCabExtractor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestCab, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			x := archive.Extractor{
				Source:      src,
				Destination: t.TempDir(),
			}
			err := x.Cab(t.Context())
			if tt.wantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			testingXTexts(t, x.Destination)
		})
	}
}
