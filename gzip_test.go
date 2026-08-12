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
		wantErr  bool
	}{
		{TestGzip1, false},
		{TestGzip2, false},
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
			be.Equal(t, c.Ext, ".gz")
			be.Equal(t, count, want)
			//testingTexts(t, c.Files...)
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

func TestGzipName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.txt.gz", "example.txt"},
		{"archive.tar.gz", "archive.tar"},
		{"/path/to/data.csv.gz", "data.csv"},
		{"file_without_ext.gz", "file_without_ext"},
		{"noextension", "noextension"},
		{".hidden.gz", ".hidden"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := archive.GzipName(tt.input); got != tt.expected {
				t.Errorf("GzipName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
