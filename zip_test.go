package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	TestImpode = "HWIMPODE.ZIP"
	TestReduce = "HWREDUCE.ZIP"
	TestShrink = "HWSHRINK.ZIP"
	TestPK1    = "PKZ110EI.ZIP"
	TestPK2    = "PKZ204EX.ZIP"
	TestPK3    = "PKZ80A1.ZIP"
)

func TestZipContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestPK1, false},
		{TestPK2, false},
		{TestPK3, false},
		// {TestImpode, true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.Zip(t.Context(), src)
			if tt.wantErr {
				be.Err(t, err)
				return
			}

			be.Err(t, err, nil)

			count := len(c.Files)
			const want = 15
			be.Equal(t, c.Ext, ".zip")
			be.Equal(t, count, want)
			testingMixes(t, c.Files...)
		})
	}
}

func TestZipExtractor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestPK1, false},
		{TestPK2, false},
		{TestPK3, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			x := archive.Extractor{
				Source:      src,
				Destination: t.TempDir(),
			}
			err := x.Zip(t.Context())
			if tt.wantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			testingXMixes(t, x.Destination)
		})
	}
}

func TestZipInfo(t *testing.T) {
	t.Parallel()

	input := []byte("file1.txt\r\nfile2.jpg\n\nsub/file3.go\r\n")
	got := archive.ZipInfo(input)

	want := []string{"file1.txt", "file2.jpg", "sub/file3.go"}
	be.Equal(t, got, want)
}
