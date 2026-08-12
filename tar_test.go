package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	TestTar1 = "BSDTAR37.TAR"
)

func TestTarContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestTar1, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.Tar(t.Context(), src)
			if tt.wantErr {
				be.Err(t, err)
				return
			}

			be.Err(t, err, nil)

			count := len(c.Files)
			const want = 3
			be.Equal(t, c.Ext, ".tar")
			be.Equal(t, count, want)
			testingTexts(t, c.Files...)
		})
	}
}

func TestTarExtractor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestTar1, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			x := archive.Extractor{
				Source:      src,
				Destination: t.TempDir(),
			}
			err := x.Tar(t.Context())
			if tt.wantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			testingXTexts(t, x.Destination)
		})
	}
}

func TestBSDTar(t *testing.T) {
	t.Parallel()

	input := []byte("folder/\r\nfolder/file1.txt\r\nfile2.jpg\n\n")
	got := archive.BSDTar(input)

	want := []string{"folder/", "folder/file1.txt", "file2.jpg"}
	be.Equal(t, got, want)
}
