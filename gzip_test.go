package archive_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	TestGzip1 = "BSDTAR37.TAR.gz" // contains three testdat?.txt files
	TestGzip2 = "GZIP113.GZ"      // contains a single file, testdat3.txt
)

func TestGzipContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		want     string
		wantErr  bool
	}{
		{TestGzip1, TestTar1, false},
		{TestGzip2, TestDat3, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.Gzip(t.Context(), src)
			if tt.wantErr {
				be.Err(t, err)
				return
			}

			be.Err(t, err, nil)

			count := len(c.Files)
			const want = 1
			be.Equal(t, c.Ext, gzx)
			be.Equal(t, count, want)
			if count > 0 {
				be.Equal(t, c.Files[0], tt.want)
			}
		})
	}
}

func TestGzipTarContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename  string
		wantCount int
		wantExt   string
		wantErr   bool
	}{
		{TestGzip1, 3, ".tar", false},
		{TestGzip2, 1, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.GzipTar(t.Context(), src)
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

func TestGzExtractor(t *testing.T) {
	t.Parallel()

	// testgzip2 which contains a single file
	dst := t.TempDir()
	src := filepath.Join(testdata, TestGzip2)
	x := archive.Extractor{
		Source:      src,
		Destination: dst,
	}
	err := x.Gzip(t.Context())
	be.Err(t, err, nil)

	want := texts[2] // testdat3.txt
	path := filepath.Join(dst, want.name)
	st, err := os.Stat(path)
	be.Err(t, err, nil)
	be.Equal(t, st.Size(), want.bytes)

	// testgzip1
	dst = t.TempDir()
	src = filepath.Join(testdata, TestGzip1)
	x = archive.Extractor{
		Source:      src,
		Destination: dst,
	}
	err = x.Gzip(t.Context())
	be.Err(t, err, nil)
	testingXTexts(t, dst)
}
