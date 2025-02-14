package archive_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/Defacto2/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: create tests for files that do not use file extensions.

func ExampleReadme() {
	name := archive.Readme("APP.ZIP", "APP.EXE", "APP.TXT", "APP.BIN", "APP.DAT", "STUFF.DAT")
	fmt.Println(name)
	// Output: APP.TXT
}

// TestData is the metadata for the example archive files found in `/testdata`
type TestData struct {
	WantErr    bool   // WantErr is true if the archive is not supported.
	Testname   string // Testname is the name of the test case to display when an error occurs.
	Filename   string // Filename is the name of the archive file in the `/testdata` directory.
	Ext        string // Ext is the expected file extension of the archive.
	cmdDos     string // cmdDos is the DOS (or Linux terminal) command used to create the archive.
	cmdName    string // cmdName is the name of the software used to create the archive.
	cmdVersion string // cmdVersion is the version of the software used to create the archive.
}

func Tests() []TestData {
	return []TestData{
		{WantErr: false,
			Testname: "7-Zip", // TODO: Read() implementation
			Filename: "7ZIP465.7Z", Ext: ".7z",
			cmdDos: "P7ZIP.EXE", cmdName: "p7zip, February 2009", cmdVersion: "4.65"},
		{WantErr: false,
			Testname: "ARC", // TODO: Read() implementation
			Filename: "ARC601.ARC", Ext: ".arc",
			cmdDos: "ARC.EXE", cmdName: "SEA ARC, January 1989", cmdVersion: "6.01"},
		{WantErr: false,
			Testname: "ARJ", // TODO: Extract() implementation
			Filename: "ARJ020B.ARJ", Ext: ".arj",
			cmdDos: "ARJ.EXE", cmdName: "Robert K Jung, December 1990", cmdVersion: "0.20 BETA"},
		{WantErr: false,
			Testname: "ARJ with no extension", // TODO: Extract() implementation
			Filename: "ARJ020B", Ext: ".arj",
			cmdDos: "ARJ.EXE", cmdName: "Robert K Jung, December 1990", cmdVersion: "0.20 BETA"},
		{WantErr: false,
			Testname: "BSD Tar", // TODO: Read() implementation
			Filename: "BSDTAR37.TAR", Ext: ".tar",
			cmdDos: "bsdtar", cmdName: "bsdtar", cmdVersion: "3.7.4"},
		{WantErr: false,
			Testname: "Gzip", // TODO: Read() implementation
			Filename: "GZIP125.GZ", Ext: ".gz",
			cmdDos: "gzip", cmdName: "Free Software Foundation, 2023", cmdVersion: "1.13"},
		{WantErr: false,
			Testname: "LHA/LZH",
			Filename: "LH113.LZH", Ext: ".lha",
			cmdDos: "LHARC.EXE", cmdName: "LHarc, May 1990", cmdVersion: "1.13"},
		{WantErr: false,
			Testname: "RAR",
			Filename: "RAR250.RAR", Ext: ".rar",
			cmdDos: "RAR.EXE", cmdName: "RAR archiver, 1999", cmdVersion: "2.50"},
		{WantErr: false,
			Testname: "Implode ZIP",
			Filename: "HWIMPODE.ZIP", Ext: ".zip",
			cmdDos: "hwzip", cmdName: "Impode", cmdVersion: "2.3"},
		{WantErr: false,
			Testname: "Reduce ZIP",
			Filename: "HWREDUCE.ZIP", Ext: ".zip",
			cmdDos: "hwzip", cmdName: "Reduce", cmdVersion: "2.3"},
		{WantErr: false,
			Testname: "Shrink ZIP",
			Filename: "HWSHRINK.ZIP", Ext: ".zip",
			cmdDos: "hwzip", cmdName: "Shrink", cmdVersion: "2.3"},
		{WantErr: true,
			Testname: "Unsupported Pak",
			Filename: "PAK100.PAK", Ext: ".pak",
			cmdDos: "PAK.EXE", cmdName: "NoGate Consulting, 1988", cmdVersion: "1.0"},
	}
}

func TestMagicExt(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() {
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join("testdata", tt.Filename)
			got, err := archive.MagicExt(src)
			if tt.WantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.Ext, got)
			}
		})
	}
}

func TestContent_Read(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() {
		const want = 3
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			got := archive.Content{}
			src := filepath.Join("testdata", tt.Filename)
			err := got.Read(src)
			if tt.WantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, want, len(got.Files))
			}
		})
	}
}

func TestExtractor_Extract(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() {
		const want = 3
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			err := archive.Extractor{
				Source:      filepath.Join("testdata", tt.Filename),
				Destination: tmp,
			}.Extract()
			if tt.WantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				n, err := helper.Count(tmp)
				require.NoError(t, err)
				assert.Equal(t, want, n)
			}
		})
	}
}

func TestExtractor_ExtractTarget(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() {
		const want = 2
		const target2, target3 = "TESTDAT2.TXT", "TESTDAT3.TXT"
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			err := archive.Extractor{
				Source:      filepath.Join("testdata", tt.Filename),
				Destination: tmp,
			}.Extract(target2, target3)
			if tt.WantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				n, err := helper.Count(tmp)
				require.NoError(t, err)
				assert.Equal(t, want, n)
			}
		})
	}
}

func TestContent_ARJ(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		filename string
		want     int
		wantExt  string
		wantErr  bool
	}{
		{"Arc", "ARC601.ARC", 0, "", true},
		{"Arj", "ARJ020B.ARJ", 3, ".arj", false},
		{"No extension Arj", "ARJ020B.ARJ", 3, ".arj", false},
		{"Impode", "HWIMPODE.ZIP", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := archive.Content{}
			src := filepath.Join("testdata", tt.filename)
			err := got.ARJ(src)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, len(got.Files))
				assert.Equal(t, tt.wantExt, got.Ext)
			}
		})
	}
}

func TestContent_LHA(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		filename string
		want     int
		wantExt  string
		wantErr  bool
	}{
		{"Arc", "ARC601.ARC", 0, "", true},
		{"LHarc", "LH113.LZH", 3, ".lha", false},
		{"Impode", "HWIMPODE.ZIP", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := archive.Content{}
			src := filepath.Join("testdata", tt.filename)
			err := got.LHA(src)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, len(got.Files))
				assert.Equal(t, tt.wantExt, got.Ext)
			}
		})
	}
}

func TestContent_RAR(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		filename string
		want     int
		wantExt  string
		wantErr  bool
	}{
		{"RAR", "RAR250.RAR", 3, ".rar", false},
		{"LHarc", "LH113.LZH", 0, "", true},
		{"Impode", "HWIMPODE.ZIP", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := archive.Content{}
			src := filepath.Join("testdata", tt.filename)
			err := got.Rar(src)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, len(got.Files))
				assert.Equal(t, tt.wantExt, got.Ext)
			}
		})
	}
}

func TestContent_Zip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		filename string
		want     int
		wantExt  string
		wantErr  bool
	}{
		{"Arc", "ARC601.ARC", 0, "", true},
		{"Arj", "ARJ020B.ARJ", 0, "", true},
		{"LHarc", "LH113.LZH", 0, "", true},
		{"RAR", "RAR250.RAR", 0, "", true},
		{"Impode", "HWIMPODE.ZIP", 3, ".zip", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := archive.Content{}
			src := filepath.Join("testdata", tt.filename)
			err := got.Zip(src)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, len(got.Files))
				assert.Equal(t, tt.wantExt, got.Ext)
			}
		})
	}
}

func TestExtractor_ARC(t *testing.T) {
	t.Parallel()
	const files = 3
	arcfile_v601_1989 := filepath.Join("testdata", "ARC601.ARC")
	dst := filepath.Join(t.TempDir())

	x := archive.Extractor{
		Source:      arcfile_v601_1989,
		Destination: dst,
	}
	err := x.ARC()
	require.NoError(t, err)

	count, err := helper.Count(dst)
	require.NoError(t, err)
	assert.Equal(t, files, count)
}

func TestExtractor_ZipHW(t *testing.T) {
	t.Parallel()
	const files = 3

	tests := []struct {
		name     string
		filename string
		want     int
		wantErr  bool
	}{
		{"Implode", "HWIMPODE.ZIP", files, false},
		{"Reduce", "HWREDUCE.ZIP", files, false},
		{"Shrink", "HWSHRINK.ZIP", files, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			err := archive.Extractor{
				Source:      filepath.Join("testdata", tt.filename),
				Destination: tmp,
			}.ZipHW()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			n, err := helper.Count(tmp)
			require.NoError(t, err)
			assert.Equal(t, tt.want, n)
		})
	}
}
