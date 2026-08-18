package archive_test

import (
	"log/slog"
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
		filename  string
		wantCount int
		wantErr   bool
	}{
		{TestPK1, 15, false},
		{TestPK2, 15, false},
		{TestPK3, 15, false},
		// {TestImpode, 3, false},
		// {TestReduce, 3, false},
		// {TestShrink, 3, false},
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
			be.Equal(t, c.Ext, ".zip")
			be.Equal(t, count, tt.wantCount)
			if tt.wantCount == 15 {
				testingMixes(t, c.Files...)
			}
			if tt.wantCount == 3 {
				testingTexts(t, c.Files...)
			}
		})
	}
}

func TestZipInfoContent(t *testing.T) {
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
			err := c.ZipInfo(t.Context(), src)
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

	sl := slog.Default()

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
			t.Log("Test ZipWithLogger on", TestPK1)

			src := filepath.Join(testdata, tt.filename)
			x := archive.Extractor{
				Source:      src,
				Destination: t.TempDir(),
			}
			err := x.ZipWithLogger(t.Context(), sl)
			if tt.wantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			testingXMixes(t, x.Destination)
		})
	}
}

func TestZipStrictExtractor(t *testing.T) {
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
			err := x.ZipStrict(t.Context())
			if tt.wantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			testingXMixes(t, x.Destination)
		})
	}
}

func TestZipUnzipExtractor(t *testing.T) {
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
			err := x.ZipUnzip(t.Context())
			if tt.wantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			testingXMixes(t, x.Destination)
		})
	}
}
