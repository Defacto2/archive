package unshrink_test

import (
	"archive/zip"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive/unshrink"
	"github.com/nalgeon/be"
)

// Test the jsummers example: https://github.com/jsummers/oldunzip/tree/master/examples

const (
	testdata    = "testdata"
	testFile    = "example-s.zip"
	extractSize = 2_048
	extractName = "S.BIN"
)

func TestUnshrink(t *testing.T) {
	unshrink.Register()
	tmp := t.TempDir()
	src := filepath.Join(testdata, testFile)

	rc, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	for n, f := range rc.File {
		fr, err := f.Open()
		if err != nil {
			t.Fatal(n, err)
		}

		s, err := filepath.Localize(f.Name)
		if err != nil {
			t.Fatal(f.Name, err)
		}
		name := filepath.Join(tmp, s)
		const wr = 578
		const wwr = 436
		dst, err := os.OpenFile(name, wr, wwr)
		if err != nil {
			fr.Close()
			t.Fatal(name, err)
		}

		size := int64(math.MaxInt64)
		if f.UncompressedSize64 < math.MaxInt64 {
			size = int64(f.UncompressedSize64)
		}

		wrote, err := io.CopyN(dst, fr, size)
		if err != nil {
			fr.Close()
			dst.Close()
			be.Err(t, err, nil)
		}
		be.Equal(t, wrote, extractSize)

		st, err := os.Stat(name)
		be.Err(t, err, nil)
		be.Equal(t, st.Name(), extractName)
		be.Equal(t, st.Size(), extractSize)
	}
}
