package archive_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/Defacto2/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	gzx = ".gz"
)

// TestData is the metadata for the example archive files found in `/testdata`.
type TestData struct {
	WantErr    bool   // WantErr is true if the archive is not supported.
	Testname   string // Testname is the name of the test case to display when an error occurs.
	Filename   string // Filename is the name of the archive file in the `/testdata` directory.
	Ext        string // Ext is the expected file extension of the archive.
	cmdDos     string // cmdDos is the DOS (or Linux terminal) command used to create the archive.
	cmdInfo    string // cmdInfo is the name of the software used to create the archive.
	cmdVersion string // cmdVersion is the version of the software used to create the archive.
}

func Tests() []TestData {
	return []TestData{
		{
			WantErr:  false,
			Testname: "7-Zip",
			Filename: "7ZIP465.7Z", Ext: ".7z",
			cmdDos: "P7ZIP.EXE", cmdInfo: "p7zip, February 2009", cmdVersion: "4.65",
		},
		{
			WantErr:  false,
			Testname: "ARC",
			Filename: "ARC601.ARC", Ext: ".arc",
			cmdDos: "ARC.EXE", cmdInfo: "SEA ARC, January 1989", cmdVersion: "6.01",
		},
		{
			WantErr:  false,
			Testname: "ARJ",
			Filename: "ARJ020B.ARJ", Ext: ".arj",
			cmdDos: "ARJ.EXE", cmdInfo: "Robert K Jung, December 1990", cmdVersion: "0.20 BETA",
		},
		{
			WantErr:  false,
			Testname: "ARJ with no extension",
			Filename: "ARJ020B", Ext: ".arj",
			cmdDos: "ARJ.EXE", cmdInfo: "Robert K Jung, December 1990", cmdVersion: "0.20 BETA",
		},
		{
			WantErr:  false,
			Testname: "BSD Tar",
			Filename: "BSDTAR37.TAR", Ext: ".tar",
			cmdDos: "bsdtar", cmdInfo: "bsdtar", cmdVersion: "3.7.4",
		},
		{
			WantErr:  false,
			Testname: "Bzip2",
			Filename: "bzip2.tar.bz2", Ext: ".bz2",
			cmdDos: "bzip2", cmdInfo: "bzip2", cmdVersion: "1.0.8",
		},
		{
			WantErr:  false,
			Testname: "Microsoft Cabinet",
			Filename: "GCAB16.CAB", Ext: ".cab",
			cmdDos: "gcab", cmdInfo: "Microsoft Cabinet using Linux gcab", cmdVersion: "1.6",
		},
		{
			WantErr:  false,
			Testname: "Gzip BSD Tar",
			Filename: "BSDTAR37.TAR.gz", Ext: ".tgz",
			cmdDos: "bsdtar", cmdInfo: "bsdtar", cmdVersion: "3.7.4",
		},
		{
			WantErr:  false,
			Testname: "Gzip",
			Filename: "GZIP113.GZ", Ext: gzx,
			cmdDos: "gzip", cmdInfo: "Free Software Foundation, 2023", cmdVersion: "1.13",
		},
		{
			WantErr:  false,
			Testname: "LHA/LZH",
			Filename: "LH113.LZH", Ext: ".lha",
			cmdDos: "LHARC.EXE", cmdInfo: "LHarc, May 1990", cmdVersion: "1.13",
		},
		{
			WantErr:  false,
			Testname: "RAR",
			Filename: "RAR250.RAR", Ext: ".rar",
			cmdDos: "RAR.EXE", cmdInfo: "RAR archiver, 1999", cmdVersion: "2.50",
		},
		{
			WantErr:  false,
			Testname: "XZ Utils",
			Filename: "XZUtils.tar.xz", Ext: ".xz",
			cmdDos: "xz", cmdInfo: "XZ Utils", cmdVersion: "5.6.2",
		},
		{
			WantErr:  false,
			Testname: "Zstandard",
			Filename: "Zstandard.tar.zst", Ext: ".zst",
			cmdDos: "zstd", cmdInfo: "Zstandard by Yann Collet", cmdVersion: "1.5.6",
		},
		{
			WantErr:  false,
			Testname: "Implode ZIP",
			Filename: "HWIMPODE.ZIP", Ext: ".zip",
			cmdDos: "hwzip", cmdInfo: "Impode", cmdVersion: "2.3",
		},
		{
			WantErr:  false,
			Testname: "Reduce ZIP",
			Filename: "HWREDUCE.ZIP", Ext: ".zip",
			cmdDos: "hwzip", cmdInfo: "Reduce", cmdVersion: "2.3",
		},
		{
			WantErr:  false,
			Testname: "Shrink ZIP",
			Filename: "HWSHRINK.ZIP", Ext: ".zip",
			cmdDos: "hwzip", cmdInfo: "Shrink", cmdVersion: "2.3",
		},
		{
			WantErr:  true,
			Testname: "Unsupported Pak",
			Filename: "PAK100.PAK", Ext: ".pak",
			cmdDos: "PAK.EXE", cmdInfo: "NoGate Consulting, 1988", cmdVersion: "1.0",
		},
		{
			WantErr:  true,
			Testname: "Not an archive",
			Filename: "TESTDAT1.TXT", Ext: ".txt",
			cmdDos: "", cmdInfo: "", cmdVersion: "",
		},
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
	for _, tt := range Tests() {
		const want = 3
		t.Run(tt.Testname, func(t *testing.T) {
			got := archive.Content{Ext: "", Files: []string{}}
			src := filepath.Join("testdata", tt.Filename)
			err := got.Read(src)
			if tt.WantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			n := len(got.Files)
			if tt.Ext == gzx {
				assert.Equal(t, 1, n)
				return
			}
			assert.Equal(t, want, n)
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
				fmt.Fprintln(os.Stderr, err)
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			n, err := helper.Count(tmp)
			require.NoError(t, err)
			if tt.Ext == gzx {
				assert.Equal(t, 1, n)
				lookupGzipExtracted(t, tmp)
				return
			}
			assert.Equal(t, want, n)
		})
	}
}

func lookupGzipExtracted(t *testing.T, tmp string) {
	t.Helper()
	items, err := os.ReadDir(tmp)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "TESTDAT3.TXT", items[0].Name())
	info, err := items[0].Info()
	require.NoError(t, err)
	assert.False(t, info.IsDir())
	assert.Equal(t, int64(81410), info.Size())
	require.NoError(t, err)
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
				return
			}
			require.NoError(t, err)
			n, err := helper.Count(tmp)
			require.NoError(t, err)
			if tt.Ext == gzx {
				assert.Equal(t, 1, n)
				return
			}
			if strings.Contains(tt.Testname, "Shrink") ||
				strings.Contains(tt.Testname, "Reduce") {
				assert.Equal(t, 3, n)
				return
			}
			assert.Equal(t, want, n)
		})
	}
}

func TestExtractor_Zips(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() {
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			if tt.Ext != ".zip" {
				return
			}
			tmp := t.TempDir()
			err := archive.Extractor{
				Source:      filepath.Join("testdata", tt.Filename),
				Destination: tmp,
			}.Zips()
			require.NoError(t, err)
			err = archive.Extractor{
				Source:      filepath.Join("testdata", tt.Filename),
				Destination: tmp,
			}.Zips("TESTDAT2.TXT", "TESTDAT3.TXT")
			switch tt.Testname {
			case "Reduce ZIP":
				require.Error(t, err)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestExtractSource(t *testing.T) {
	for _, tt := range Tests() {
		t.Run(tt.Testname, func(t *testing.T) {
			src := filepath.Join("testdata", tt.Filename)
			got, err := archive.ExtractSource(src, "tester")
			if tt.WantErr && tt.Ext != ".txt" {
				require.Error(t, err, tt.Filename)
				return
			}
			require.NoError(t, err)
			_, err = os.Stat(got)
			require.NoError(t, err)
		})
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() {
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join("testdata", tt.Filename)
			got, err := archive.List(src, tt.Filename)
			if tt.WantErr && tt.Ext != ".txt" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, got)
		})
	}
}

func TestInvalidFormats(t *testing.T) { //nolint:cyclop
	t.Parallel()
	for _, tt := range Tests() {
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join("testdata", tt.Filename)
			c := archive.Content{
				Ext:   "",
				Files: []string{},
			}
			tmp := t.TempDir()
			if !strings.EqualFold(tt.Ext, ".7z") {
				err := c.Zip7(src)
				require.Error(t, err, tt.Filename)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Zip7()
				require.Error(t, err, tt.Filename)
			}
			if !strings.EqualFold(tt.Ext, ".arc") {
				err := c.ARC(src)
				require.Error(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.ARC()
				require.Error(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".arj") {
				err := c.ARJ(src)
				require.Error(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.ARJ()
				require.Error(t, err)
			}
			if !strings.EqualFold(tt.Ext, gzx) &&
				!strings.EqualFold(tt.Ext, ".tgz") {
				err := c.Gzip(src)
				require.Error(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Gzip()
				require.Error(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".lha") {
				err := c.LHA(src)
				require.Error(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.LHA()
				require.Error(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".rar") {
				err := c.Rar(src)
				require.Error(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Rar()
				require.Error(t, err)
			}
			skipExts := []string{".7z", ".bz2", ".cab", ".lha", ".tar", ".tgz", ".xz", ".zst", ".zip"}
			if !slices.Contains(skipExts, strings.ToLower(tt.Ext)) {
				err := c.Tar(src)
				require.Error(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Tar()
				require.Error(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".zip") {
				err := c.Zip(src)
				require.Error(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Zip()
				require.Error(t, err)
			}
		})
	}
}

func TestHardLink(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		require string
		src     string // in tests, make a relative path.
		want    string
		wantErr bool
	}{
		{
			"Missing ARJ extension", ".arj", "ARCHIVE",
			"ARCHIVE.arj", false,
		},
		{
			"Missing TAR GZ extension", ".tar.gz", "ARCHIVE",
			"ARCHIVE.tar.gz", false,
		},
		{
			"Not a valid extension", "arj", "ARCHIVE",
			"ARCHIVE.arj", true,
		},
		{
			"Using ARJ extension", ".arj", "ARCHIVE.arj",
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join(t.TempDir(), tt.src)
			err := helper.Touch(src)
			require.NoError(t, err)

			got, err := archive.HardLink(tt.require, src)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.want == "" {
				assert.Empty(t, got)
				return
			}
			assert.True(t, strings.HasSuffix(got, tt.want))
		})
	}
}

func TestGzipName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			"Filename with extension", "ARCHIVE.tar.gz",
			"ARCHIVE.tar",
		},
		{
			"Filename without extension", "ARCHIVE.gz",
			"ARCHIVE",
		},
		{
			"Filename with multiple dots", "ARCHIVE.tar.gz.gz",
			"ARCHIVE.tar.gz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := archive.GzipName(tt.src)
			assert.Equal(t, tt.want, got)
		})
	}
}
