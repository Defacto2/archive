package archive_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	TestBz2  = "bzip2.tar.bz2"
	TestZstd = "Zstandard.tar.zst"
)

func TestBz2Content(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantExt  string
		wantName string
	}{
		{TestBz2, ".bz2", "bzip2.tar"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.Bz2(t.Context(), src)
			be.Err(t, err, nil)

			be.Equal(t, c.Ext, tt.wantExt)
			wantFiles := 1
			got := len(c.Files)
			be.Equal(t, got, wantFiles)
			if got > 0 {
				be.Equal(t, c.Files[0], tt.wantName)
			}
		})
	}
}

func TestBz2Extractor(t *testing.T) {
	t.Parallel()

	dst := t.TempDir()
	src := filepath.Join(testdata, TestBz2)
	x := archive.Extractor{
		Source:      src,
		Destination: dst,
	}
	err := x.Bz2(t.Context())
	be.Err(t, err, nil)

	ext := filepath.Ext(src)
	path := strings.TrimSuffix(src, ext)
	base := filepath.Base(path)
	name := filepath.Join(dst, base)
	st, err := os.Stat(name)
	be.Err(t, err, nil)
	if err != nil {
		t.Fatal(name, err)
	}
	be.Equal(t, st.Size(), 87_040)
}

func TestBz2TarContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename  string
		wantCount int
		wantExt   string
		wantErr   bool
	}{
		{TestBz2, 3, ".bz2", false},
		//	{TestGzip2, 1, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.Bz2Tar(t.Context(), src)
			if tt.wantErr {
				be.Err(t, err)
				return
			}

			be.Err(t, err, nil)

			count := len(c.Files)
			be.Equal(t, c.Ext, tt.wantExt)
			be.Equal(t, count, tt.wantCount)
		})
	}
}
