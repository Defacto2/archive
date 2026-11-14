package arc_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive/arc"
	"github.com/nalgeon/be"
)

const (
	testdata  = "../testdata"
	arc601    = "ARC601.ARC"
	arcOld    = "ARC_OLD.ARC"
	arcStored = "ARC_STOR.ARC"
	arcPacked = "ARC_PACK.ARC"
	arcSque   = "ARC_SQUE.ARC"
	arcCru5   = "ARC_CRU5.ARC"
	arcCru6   = "ARC_CRU6.ARC"
	arcCru7   = "ARC_CRU7.ARC"
	arcSquash = "ARC_SQUA.ARC"
)

func TestReader(t *testing.T) {
	t.Parallel()

	r, f, err := arc.OpenReader(filepath.Join(testdata, arc601))
	be.Err(t, err, nil)
	defer f.Close()

	be.Equal(t, len(r.Files), 3)

	expected := map[string]struct {
		size uint32
		crc  uint16
	}{
		"TESTDAT1.TXT": {2009, 0xE60D},
		"TESTDAT2.TXT": {469, 0xD0AF},
		"TESTDAT3.TXT": {81410, 0xE2C5},
	}

	for _, entry := range r.Files {
		exp, ok := expected[entry.Name]
		be.True(t, ok)
		be.Equal(t, entry.UncompressedSize, exp.size)
		be.Equal(t, entry.CRC, exp.crc)

		rc, oErr := entry.Open()
		be.Err(t, oErr, nil)
		buf := make([]byte, entry.UncompressedSize)
		n, rErr := rc.Read(buf)
		_ = rc.Close()
		be.Err(t, rErr, nil)
		be.Equal(t, int64(n), int64(exp.size))
	}
}

func TestMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		method  byte
		entries int
	}{
		{"Old Stored (Method 1)", arcOld, arc.MethodOldStored, 1},
		{"Stored (Method 2)", arcStored, arc.MethodStored, 3},
		{"Packed (Method 3)", arcPacked, arc.MethodPacked, 1},
		{"Squeezed (Method 4)", arcSque, arc.MethodSqueezed, 1},
		{"Old Crunched (Method 5)", arcCru5, arc.MethodCrunched5, 1},
		{"Old Crunched RLE (Method 6)", arcCru6, arc.MethodCrunched6, 1},
		{"Crunched New Hash (Method 7)", arcCru7, arc.MethodCrunched7, 1},
		{"Crunched (Method 8)", arc601, arc.MethodCrunched8, 3},
		{"Squashed (Method 9)", arcSquash, arc.MethodSquashed, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(testdata, tt.file)
			r, f, err := arc.OpenReader(path)
			be.Err(t, err, nil)
			defer f.Close()

			be.Equal(t, len(r.Files), tt.entries)
			be.Equal(t, r.Files[0].Method, tt.method)

			dest := t.TempDir()
			err = r.ExtractAll(dest)
			be.Err(t, err, nil)

			for _, entry := range r.Files {
				st, sErr := os.Stat(filepath.Join(dest, entry.Name))
				be.Err(t, sErr, nil)
				be.Equal(t, st.Size(), int64(entry.UncompressedSize))
			}
		})
	}
}
