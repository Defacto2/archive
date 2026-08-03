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
	"github.com/nalgeon/be"
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

func Tests() []TestData { //nolint:funlen
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
			cmdDos: "ARcnt.EXE", cmdInfo: "SEA ARC, January 1989", cmdVersion: "6.01",
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
			cmdDos: "LHARcnt.EXE", cmdInfo: "LHarc, May 1990", cmdVersion: "1.13",
		},
		{
			WantErr:  false,
			Testname: "LHA/LH0",
			Filename: "LH0.LZH", Ext: ".lha",
			cmdDos: "LHARcnt.EXE", cmdInfo: "LHarc, May 1990", cmdVersion: "1.13",
		},
		{
			WantErr:  false,
			Testname: "LHA/LH5",
			Filename: "LH5.LZH", Ext: ".lha",
			cmdDos: "LHARcnt.EXE", cmdInfo: "LHarc, May 1990", cmdVersion: "1.13",
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
			WantErr:  false,
			Testname: "Pak",
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

func TestLsar(t *testing.T) {
	t.Parallel()
	noSupport := []string{"Zstandard"}
	for _, tt := range Tests() { //nolint:varnamelen
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
	noSupport := []string{"Reduce ZIP", "Shrink ZIP", "Zstandard"}
	for _, tt := range Tests() { //nolint:varnamelen
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

func TestMagicExt(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() { //nolint:varnamelen
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join("testdata", tt.Filename)
			got, err := archive.MagicExt(t.Context(), src)
			if tt.WantErr {
				be.Err(t, err)
			} else {
				be.Err(t, err, nil)
				be.Equal(t, got, tt.Ext)
			}
		})
	}
}

func TestContent_Read(t *testing.T) {
	for _, tt := range Tests() { //nolint:varnamelen
		const want = 3
		t.Run(tt.Testname, func(t *testing.T) {
			arch := archive.Content{Ext: "", Files: []string{}}
			src := filepath.Join("testdata", tt.Filename)
			err := arch.Read(t.Context(), src)
			if tt.WantErr {
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)
			got := len(arch.Files)
			if tt.Ext == gzx {
				be.Equal(t, got, 1)
				return
			}
			be.Equal(t, got, want)
		})
	}
}

func TestExtractor_Extract(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() { //nolint:varnamelen
		const want = 3
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			err := archive.Extractor{
				Source:      filepath.Join("testdata", tt.Filename),
				Destination: tmp,
			}.Extract(t.Context())
			if tt.WantErr {
				fmt.Fprintln(os.Stderr, err)
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)
			got, err := helper.Count(tmp)
			be.Err(t, err, nil)
			if tt.Ext == gzx {
				be.Equal(t, got, 1)
				lookupGzipExtracted(t, tmp)
				return
			}
			be.Equal(t, got, want)
		})
	}
}

func lookupGzipExtracted(t *testing.T, tmp string) {
	t.Helper()
	items, err := os.ReadDir(tmp)
	be.Err(t, err, nil)
	be.Equal(t, len(items), 1)
	be.Equal(t, "TESTDAT3.TXT", items[0].Name())
	info, err := items[0].Info()
	be.Err(t, err, nil)
	be.True(t, !info.IsDir())
	be.Equal(t, int64(81410), info.Size())
	be.Err(t, err, nil)
}

func TestExtractor_ExtractTarget(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() { //nolint:varnamelen
		const want = 2
		const target2, target3 = "TESTDAT2.TXT", "TESTDAT3.TXT"
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			err := archive.Extractor{
				Source:      filepath.Join("testdata", tt.Filename),
				Destination: tmp,
			}.Extract(t.Context(), target2, target3)
			if tt.WantErr {
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)
			got, err := helper.Count(tmp)
			be.Err(t, err, nil)
			if tt.Ext == gzx {
				be.Equal(t, got, 1)
				return
			}
			if strings.Contains(tt.Testname, "Shrink") ||
				strings.Contains(tt.Testname, "Reduce") {
				be.Equal(t, got, 3)
				return
			}
			be.Equal(t, got, want)
		})
	}
}

func TestExtractor_Zips(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() { //nolint:varnamelen
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			if tt.Ext != ".zip" {
				return
			}
			tmp := t.TempDir()
			err := archive.Extractor{
				Source:      filepath.Join("testdata", tt.Filename),
				Destination: tmp,
			}.Zips(t.Context())
			be.Err(t, err, nil)
			err = archive.Extractor{
				Source:      filepath.Join("testdata", tt.Filename),
				Destination: tmp,
			}.Zips(t.Context(), "TESTDAT2.TXT", "TESTDAT3.TXT")
			switch tt.Testname {
			case "Reduce ZIP":
				be.Err(t, err)
			default:
				be.Err(t, err, nil)
			}
		})
	}
}

func TestExtractSource(t *testing.T) {
	for _, tt := range Tests() {
		t.Run(tt.Testname, func(t *testing.T) {
			src := filepath.Join("testdata", tt.Filename)
			got, err := archive.ExtractSource(t.Context(), src, "tester")
			if tt.WantErr && tt.Ext != ".txt" {
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)
			_, err = os.Stat(got)
			be.Err(t, err, nil)
		})
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	for _, tt := range Tests() {
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join("testdata", tt.Filename)
			got, err := archive.List(t.Context(), src, tt.Filename)
			if tt.WantErr && tt.Ext != ".txt" {
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)
			notEmpty := len(got) > 0
			be.True(t, notEmpty)
		})
	}
}

func TestInvalidFormats(t *testing.T) { //nolint:funlen
	t.Parallel()
	for _, tt := range Tests() { //nolint:varnamelen
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join("testdata", tt.Filename)
			cnt := archive.Content{
				Ext:   "",
				Files: []string{},
			}
			tmp := t.TempDir()
			skipExts := []string{
				".7z", ".arc", ".arj", ".bz2", ".cab", ".gz", ".lha", ".pak", ".rar", ".tar", ".tgz", ".xz", ".zst", ".zip",
			}
			if !slices.Contains(skipExts, strings.ToLower(tt.Ext)) {
				err := cnt.Lsar(t.Context(), src)
				be.Err(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Unar(t.Context())
				be.Err(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".7z") {
				err := cnt.Zip7(t.Context(), src)
				be.Err(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Zip7(t.Context())
				be.Err(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".arc") {
				err := cnt.ARC(t.Context(), src)
				be.Err(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.ARC(t.Context())
				be.Err(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".arj") {
				err := cnt.ARJ(t.Context(), src)
				be.Err(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.ARJ(t.Context())
				be.Err(t, err)
			}
			if !strings.EqualFold(tt.Ext, gzx) &&
				!strings.EqualFold(tt.Ext, ".tgz") {
				err := cnt.Gzip(t.Context(), src)
				be.Err(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Gzip(t.Context())
				be.Err(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".lha") {
				err := cnt.LHA(t.Context(), src)
				be.Err(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.LHA(t.Context())
				be.Err(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".rar") {
				err := cnt.Rar(t.Context(), src)
				be.Err(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Rar(t.Context())
				be.Err(t, err)
			}
			skipExts = []string{".7z", ".bz2", ".cab", ".lha", ".tar", ".tgz", ".xz", ".zst", ".zip"}
			if !slices.Contains(skipExts, strings.ToLower(tt.Ext)) {
				err := cnt.Tar(t.Context(), src)
				be.Err(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Tar(t.Context())
				be.Err(t, err)
			}
			if !strings.EqualFold(tt.Ext, ".zip") {
				err := cnt.Zip(t.Context(), src)
				be.Err(t, err)
				x := archive.Extractor{Source: src, Destination: tmp}
				err = x.Zip(t.Context())
				be.Err(t, err)
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
	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join(t.TempDir(), tt.src)
			err := helper.Touch(src)
			be.Err(t, err, nil)
			got, err := archive.HardLink(tt.require, src)
			if tt.wantErr {
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)
			if tt.want == "" {
				be.Equal(t, got, "")
				return
			}
			be.True(t, strings.HasSuffix(got, tt.want))
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
			be.Equal(t, got, tt.want)
		})
	}
}
