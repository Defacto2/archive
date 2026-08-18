package archive_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	TestXZ = "XZUtils.tar.xz"
)

func TestLsar(t *testing.T) {
	t.Parallel()
	noSupport := []string{"Zstandard Tar", "Zstandard"}
	for _, tt := range Tests(t) { //nolint:varnamelen
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join("testdata", tt.Filename)
			var content archive.Content
			if slices.Contains(noSupport, tt.Testname) {
				tt.WantErr = true
			}
			err := content.Lsar(t.Context(), src)
			if tt.WantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
			ext := content.Ext
			be.Equal(t, ext, "")
		})
	}
}

func TestUnar(t *testing.T) {
	t.Parallel()
	noSupport := []string{"Reduce ZIP", "Shrink ZIP", "Zstandard Tar", "Zstandard"}
	for _, tt := range Tests(t) { //nolint:varnamelen
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join("testdata", tt.Filename)
			extract := archive.Extractor{
				Source:      src,
				Destination: t.ArtifactDir(),
			}
			if slices.Contains(noSupport, tt.Testname) {
				tt.WantErr = true
			}
			err := extract.Unar(t.Context())
			if tt.WantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
			}
		})
	}
}
