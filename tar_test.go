package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

// to create new tar files:
// $ bsdtar --create --format "cpio|pax|shar|ustar" --file bsd_new.tar TESTDAT*.TXT
// $ bsdtar --create --xz --file bsd_new.txz TESTDAT*.TXT
// $ other compression options include --lrzip, --lz4, --zstd --lzma --lzop --gzip

const (
	TestTar1 = "BSDTAR37.TAR"
	TestTar2 = "bsd_cpio.tar"
	TestTar3 = "bsd_pax.tar"
	TestTar4 = "bsd_shar.tar"
	TestTar5 = "bsd_ustar.tar"
)

func TestTarContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestTar1, false},
		{TestTar2, true},
		{TestTar3, false},
		{TestTar4, true},
		{TestTar5, false},
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
		{TestTar2, true},
		{TestTar3, false},
		{TestTar4, true},
		{TestTar5, false},
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
				return
			}

			be.Err(t, err, nil)
			testingXTexts(t, x.Destination)
		})
	}
}
