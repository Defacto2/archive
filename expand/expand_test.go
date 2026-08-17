package expand_test

import (
	"archive/zip"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive/expand"
	"github.com/nalgeon/be"
)

// Test the jsummers example: https://github.com/jsummers/oldunzip/tree/master/examples

const (
	testdata    = "testdata"
	testFile    = "example-r.zip"
	extractSize = 2_048
	extractName = "R.BIN"
	extractCRC  = 0x8314ddd9
)

func TestExpand(t *testing.T) {
	expand.Register()
	tmp := t.TempDir()
	src := filepath.Join(testdata, testFile)

	rc, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	for n, f := range rc.File {
		fr, err := f.Open()
		if err != nil {
			t.Fatal(n, err)
		}

		s, err := filepath.Localize(f.Name)
		if err != nil {
			fr.Close()
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
		fr.Close()
		dst.Close()
		if err != nil {
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

func TestExpandSmallBuffers(t *testing.T) {
	expand.Register()
	src := filepath.Join(testdata, testFile)

	rc, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	if len(rc.File) == 0 {
		t.Fatal("no files in archive")
	}

	fr, err := rc.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer fr.Close()

	// Read in 1-byte increments to verify incremental decompression
	buf := make([]byte, 1)
	hasher := crc32.NewIEEE()
	var total int

	for {
		n, err := fr.Read(buf)
		if n > 0 {
			total += n
			hasher.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error reading: %v", err)
		}
	}

	be.Equal(t, total, extractSize)
	be.Equal(t, hasher.Sum32(), uint32(extractCRC))
}

func TestExpandConstants(t *testing.T) {
	be.Equal(t, expand.Expand2, uint16(2))
	be.Equal(t, expand.Expand3, uint16(3))
	be.Equal(t, expand.Expand4, uint16(4))
	be.Equal(t, expand.Expand5, uint16(5))
}
