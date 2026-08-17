package unshrink_test

import (
	"archive/zip"
	"hash/crc32"
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
	extractCRC  = 0x8314ddd9
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
		dst, err := os.Create(name)
		if err != nil {
			fr.Close()
			t.Fatal(name, err)
		}

		size := int64(math.MaxInt64)
		if f.UncompressedSize64 < math.MaxInt64 {
			size = int64(f.UncompressedSize64)
		}

		hasher := crc32.NewIEEE()
		mw := io.MultiWriter(dst, hasher)

		wrote, err := io.CopyN(mw, fr, size)
		if err != nil {
			fr.Close()
			dst.Close()
			t.Fatal(name, wrote, err)
		}
		be.Equal(t, wrote, extractSize)
		be.Equal(t, hasher.Sum32(), uint32(extractCRC))

		st, err := os.Stat(name)
		be.Err(t, err, nil)
		be.Equal(t, st.Name(), extractName)
		be.Equal(t, st.Size(), extractSize)
	}
}
