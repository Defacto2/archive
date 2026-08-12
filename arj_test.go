package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	TestArj1 = "ARJ020B"
	TestArj2 = "ARJ020B.ARJ"
)

func TestArjContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestArj1, false},
		{TestArj2, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			var c archive.Content
			err := c.ARJ(t.Context(), src)
			if tt.wantErr {
				be.Err(t, err)
				return
			}

			be.Err(t, err, nil)

			count := len(c.Files)
			const want = 3
			be.Equal(t, c.Ext, ".arj")
			be.Equal(t, count, want)
			testingTexts(t, c.Files...)
		})
	}
}

func TestArjExtractor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		wantErr  bool
	}{
		{TestArj1, false},
		{TestArj2, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			src := filepath.Join(testdata, tt.filename)
			x := archive.Extractor{
				Source:      src,
				Destination: t.TempDir(),
			}
			err := x.ARJ(t.Context())
			if tt.wantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			testingXTexts(t, x.Destination)
		})
	}
}

func TestArjFiles(t *testing.T) {
	t.Parallel()

	const output1 = `
ARJ 2.78 Copyright (c) 1998-2004 Robert K Jung

Filename       Original Compressed Ratio DateTime modified Attributes/GUA BPMGS
------------ ---------- ---------- ----- ----------------- -------------- -----
TESTDAT1.TXT       2009        889 0.443 25-02-14 13:21:10                  1
TESTDAT2.TXT        469        266 0.567 25-02-14 13:17:34                  1
TESTDAT3.TXT      81410      22438 0.276 25-02-14 13:21:02                  1
------------ ---------- ---------- -----
      3 files      83888      23593 0.281
`

	files := archive.ARJs([]byte(output1))
	count := len(files)

	const want = 3
	be.Equal(t, count, want)
	if count >= want {
		be.Equal(t, files[0], "TESTDAT1.TXT")
		be.Equal(t, files[1], "TESTDAT2.TXT")
		be.Equal(t, files[2], "TESTDAT3.TXT")
	}

	const output2 = `
	 Filename       Original Compressed Ratio DateTime modified Attributes/GUA BPMGS
	 ------------ ---------- ---------- ----- ----------------- -------------- -----
	 TESTDAT1.TXT       2009        889 0.443 25-02-14 13:21:10                  1
	 TESTDAT2.TXT        469        266 0.567 25-02-14 13:17:34                  1
	 TESTDAT3.TXT      81410      22438 0.276 25-02-14 13:21:02                  1
	 ------------ ---------- ---------- -----
	      3 files      83888      23593 0.281
		`

	files = archive.ARJs([]byte(output2))
	count = len(files)

	be.Equal(t, count, want)
	if count >= want {
		be.Equal(t, files[0], "TESTDAT1.TXT")
		be.Equal(t, files[1], "TESTDAT2.TXT")
		be.Equal(t, files[2], "TESTDAT3.TXT")
	}
}
