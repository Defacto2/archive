package explode_test

import (
	"archive/zip"
	"errors"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive/explode"
	"github.com/nalgeon/be"
)

const (
	testdata    = "testdata"
	testFile    = "example-i.zip"
	extractSize = 2_048
	extractName = "I.BIN"
	extractCRC  = 0x8314ddd9
)

func TestExplode(t *testing.T) {
	explode.Register()
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
			t.Fatalf("%s wrote %d err: %v", name, wrote, err)
		}
		be.Equal(t, wrote, extractSize)
		be.Equal(t, hasher.Sum32(), uint32(extractCRC))

		st, err := os.Stat(name)
		be.Err(t, err, nil)
		be.Equal(t, st.Name(), extractName)
		be.Equal(t, st.Size(), extractSize)
	}
}

func TestNewReader_Chunked(t *testing.T) {
	src := filepath.Join(testdata, testFile)
	rc, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	for _, chunkSize := range []int{1, 3, 7, 64, 512, 4096} {
		f := rc.File[0]
		fr, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}

		hasher := crc32.NewIEEE()
		buf := make([]byte, chunkSize)
		var total int64

		for {
			n, err := fr.Read(buf)
			if n > 0 {
				total += int64(n)
				hasher.Write(buf[:n])
			}
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				fr.Close()
				t.Fatalf("chunkSize=%d read err: %v", chunkSize, err)
			}
		}
		fr.Close()

		be.Equal(t, total, extractSize)
		be.Equal(t, hasher.Sum32(), uint32(extractCRC))
	}
}

func BenchmarkExplode(b *testing.B) {
	explode.Register()
	src := filepath.Join(testdata, testFile)
	rc, err := zip.OpenReader(src)
	if err != nil {
		b.Fatal(err)
	}
	defer rc.Close()

	f := rc.File[0]
	buf := make([]byte, extractSize)

	b.ResetTimer()
	b.SetBytes(extractSize)

	for b.Loop() {
		fr, err := f.Open()
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.ReadFull(fr, buf)
		fr.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}
