package pklite_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Defacto2/archive/pklite"
	"github.com/nalgeon/be"
)

// The test files are in the testdata directory of the archive package.
// D2.COM is the uncompressed original of the D2_V1B.COM and D2_V115.COM files,
// and LOR.EXE is the original of LOR_V115.EXE.
const (
	original = "D2.COM"
	beta     = "D2_V1B.COM"
	v115     = "D2_V115.COM"
	exe      = "LOR.EXE"
	exe115   = "LOR_V115.EXE"
	exeV3    = "LOR_V3.EXE"
	exeV3A   = "LOR_V3A.EXE"
	exeV1E   = "LOR_V1E.EXE"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", name))
	be.Err(t, err, nil)
	return b
}

func TestDecompressCOM(t *testing.T) {
	t.Parallel()
	want := read(t, original)
	be.Equal(t, len(want), 4176)
	for _, name := range []string{beta, v115} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data := read(t, name)
			be.True(t, pklite.IsCOM(data))
			be.True(t, !pklite.IsEXE(data))
			out, err := pklite.Decompress(data)
			be.Err(t, err, nil)
			be.True(t, bytes.Equal(out, want)) // the result is identical to the original
		})
	}
}

func TestDecompressEXE(t *testing.T) {
	t.Parallel()
	want := read(t, exe)
	data := read(t, exe115)
	be.True(t, pklite.IsEXE(data))
	be.True(t, !pklite.IsCOM(data))
	be.True(t, !pklite.IsEXE(want))
	out, err := pklite.Decompress(data)
	be.Err(t, err, nil)
	be.True(t, bytes.Equal(out, want)) // the result is identical to the original
}

// The same PKLITE as LOR_V3.EXE, with the -a flag, stores the original header.
func TestDecompressEXEAlways(t *testing.T) {
	t.Parallel()
	data := read(t, exeV3A)
	be.True(t, pklite.IsEXE(data))
	out, err := pklite.Decompress(data)
	be.Err(t, err, nil)
	be.True(t, bytes.Equal(out, read(t, exe))) // the result is identical to the original
}

// mz returns the registers, relocation table and code of an EXE.
func mz(t *testing.T, b []byte) (regs [5]uint16, relocs, code []byte) {
	t.Helper()
	le := binary.LittleEndian
	be.True(t, bytes.HasPrefix(b, []byte("MZ")))
	for i, off := range []int{14, 16, 20, 22, 10} { // ss, sp, ip, cs, minalloc
		regs[i] = le.Uint16(b[off:])
	}
	pos, n := int(le.Uint16(b[24:])), int(le.Uint16(b[6:]))
	return regs, b[pos : pos+4*n], b[int(le.Uint16(b[8:]))*16:]
}

// linear returns the sorted linear addresses of a relocation table.
// The long relocation table of the extra compression uses different segment and
// offset pairs than the original, but they refer to the same addresses.
func linear(table []byte) []int {
	addrs := make([]int, 0, len(table)/4)
	for i := 0; i+4 <= len(table); i += 4 {
		off := int(binary.LittleEndian.Uint16(table[i:]))
		seg := int(binary.LittleEndian.Uint16(table[i+2:]))
		addrs = append(addrs, seg*16+off)
	}
	slices.Sort(addrs)
	return addrs
}

// The extra compression files do not store the original header, so it is rebuilt.
// The header size, padding and memory allocation may differ, but not the program.
func TestDecompressEXEExtra(t *testing.T) {
	t.Parallel()
	want := read(t, exe)
	for _, name := range []string{exeV3, exeV1E} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data := read(t, name)
			be.True(t, pklite.IsEXE(data))
			out, err := pklite.Decompress(data)
			be.Err(t, err, nil)

			wantRegs, wantRelocs, wantCode := mz(t, want)
			regs, relocs, code := mz(t, out)
			be.Equal(t, regs[:4], wantRegs[:4])
			be.Equal(t, linear(relocs), linear(wantRelocs))
			be.True(t, bytes.Equal(code, wantCode))
			// the allocation must not be less than the original
			be.True(t, regs[4] >= wantRegs[4])
		})
	}
}

func TestRead(t *testing.T) {
	t.Parallel()
	out, err := pklite.Read(bytes.NewReader(read(t, v115)))
	be.Err(t, err, nil)
	be.Equal(t, len(out), 4176)
}

func TestNotPKLITE(t *testing.T) {
	t.Parallel()
	data := read(t, original)
	be.True(t, !pklite.IsCOM(data))
	be.True(t, !pklite.IsEXE(data))
	_, err := pklite.Decompress(data)
	be.True(t, errors.Is(err, pklite.ErrNotPKLITE))
	_, err = pklite.Decompress(nil)
	be.True(t, errors.Is(err, pklite.ErrNotPKLITE))
}

// Truncated and damaged inputs must return an error, and never panic.
func TestCorrupt(t *testing.T) {
	t.Parallel()
	for _, name := range []string{beta, v115, exe115, exeV3, exeV3A, exeV1E} {
		data := read(t, name)
		for n := range len(data) {
			_, _ = pklite.Decompress(data[:n])
		}
		for i := range len(data) {
			bad := bytes.Clone(data)
			bad[i] ^= 0xff
			_, _ = pklite.Decompress(bad)
		}
	}
	data := read(t, v115)
	_, err := pklite.Decompress(data[:len(data)-300])
	be.True(t, err != nil)
}

func FuzzDecompress(f *testing.F) {
	for _, n := range []string{beta, v115, exe115, exeV3, exeV3A, exeV1E} {
		b, err := os.ReadFile(filepath.Join("..", "testdata", n))
		if err == nil {
			f.Add(b)
		}
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = pklite.Decompress(data)
	})
}
