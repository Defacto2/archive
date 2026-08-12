package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	TestLZH1 = "LH0.LZH"
	TestLZH2 = "LH113.LZH"
	TestLZH3 = "LH5.LZH"
)

func TestLhaContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestLZH1, false},
		{TestLZH2, false},
		{TestLZH3, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.LHA(t.Context(), src)
			if tt.wantErr {
				be.Err(t, err)
				return
			}

			be.Err(t, err, nil)

			count := len(c.Files)
			const want = 3
			be.Equal(t, c.Ext, ".lha")
			be.Equal(t, count, want)
			testingLower(t, c.Files...)
		})
	}
}

func TestLhaExtractor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestLZH1, false},
		{TestLZH2, false},
		{TestLZH3, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			x := archive.Extractor{
				Source:      src,
				Destination: t.TempDir(),
			}
			err := x.LHA(t.Context())
			if tt.wantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			testingXLower(t, x.Destination)
		})
	}
}

func TestLHAFiles(t *testing.T) {
	t.Parallel()

	const output1 = `
PERMSSN    UID  GID      SIZE  RATIO      STAMP            NAME
---------- ----------- ------- ------ ------------ --------------------
[generic]                 2009  48.8% Feb 14 13:21 testdat1.txt
[generic]                  469  66.5% Feb 14 13:17 testdat2.txt
[generic]                81410  29.5% Feb 14 13:21 testdat3.txt
---------- ----------- ------- ------ ------------ --------------------
 Total         3 files   83888  30.2% Feb 14 07:19
`

	files := archive.LHAs([]byte(output1))
	count := len(files)

	const want = 3
	be.Equal(t, count, want)
	if count >= want {
		be.Equal(t, files[0], "testdat1.txt")
		be.Equal(t, files[1], "testdat2.txt")
		be.Equal(t, files[2], "testdat3.txt")
	}

	const output2 = `
PERMSSN    UID  GID      SIZE  RATIO      STAMP            NAME
---------- ----------- ------- ------ ------------ --------------------
[generic]                 1024  50.0% Feb 14 13:21 docs/my report.pdf
[generic]                 2048  40.0% Feb 14 13:22 images/photo.png
---------- ----------- ------- ------ ------------ --------------------
 Total         2 files    3072  43.3% Feb 14 13:22
		`

	files = archive.LHAs([]byte(output2))
	count = len(files)

	const want2 = 2
	be.Equal(t, count, want2)
	if count >= want2 {
		be.Equal(t, files[0], "docs/my report.pdf")
		be.Equal(t, files[1], "images/photo.png")
	}
}
