package lbr_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive/lbr"
	"github.com/nalgeon/be"
)

const (
	testdata = "../testdata"
	testLBR  = "TEST.LBR"
)

func TestReader(t *testing.T) {
	t.Parallel()

	r, f, err := lbr.OpenReader(filepath.Join(testdata, testLBR))
	be.Err(t, err, nil)
	defer f.Close()

	be.Equal(t, len(r.Files), 3)

	expected := map[string]struct {
		size int64
		crc  uint16
	}{
		"TESTDAT1.TXT": {2009, 0xF7D5},
		"TESTDAT2.TXT": {469, 0xD003},
		"TESTDAT3.TXT": {81410, 0x617F},
	}

	for _, entry := range r.Files {
		exp, ok := expected[entry.Name]
		be.True(t, ok)
		be.Equal(t, entry.Size, exp.size)
		be.Equal(t, entry.CRC, exp.crc)

		rc, oErr := entry.Open()
		be.Err(t, oErr, nil)
		data, rErr := io.ReadAll(rc)
		_ = rc.Close()
		be.Err(t, rErr, nil)
		be.Equal(t, int64(len(data)), exp.size)

		orig, fErr := os.ReadFile(filepath.Join(testdata, entry.Name))
		be.Err(t, fErr, nil)
		be.Equal(t, data, orig)
	}
}

func TestExtractAll(t *testing.T) {
	t.Parallel()

	r, f, err := lbr.OpenReader(filepath.Join(testdata, testLBR))
	be.Err(t, err, nil)
	defer f.Close()

	dest := t.TempDir()
	err = r.ExtractAll(dest)
	be.Err(t, err, nil)

	for _, entry := range r.Files {
		st, sErr := os.Stat(filepath.Join(dest, entry.Name))
		be.Err(t, sErr, nil)
		be.Equal(t, st.Size(), entry.Size)
	}
}

func TestExtractTarget(t *testing.T) {
	t.Parallel()

	r, f, err := lbr.OpenReader(filepath.Join(testdata, testLBR))
	be.Err(t, err, nil)
	defer f.Close()

	dest := t.TempDir()
	err = r.ExtractAll(dest, "testdat2.txt")
	be.Err(t, err, nil)

	entries, err := os.ReadDir(dest)
	be.Err(t, err, nil)
	be.Equal(t, len(entries), 1)
	be.Equal(t, entries[0].Name(), "TESTDAT2.TXT")
}
