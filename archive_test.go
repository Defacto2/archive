package archive_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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

// testdata returns the absolute path to the archive/testdata directory.
var testdata = func() string { //nolint:gochecknoglobals
	const format = "testdata %s: %v"
	const path = "testdata"
	dir, err := filepath.Abs(path)
	if err != nil {
		panic(fmt.Sprintf(format, "absolute path failed", err))
	}
	st, err := os.Stat(dir)
	if err != nil {
		panic(fmt.Sprintf(format, "missing or unreadable "+dir, err))
	}
	if !st.IsDir() {
		panic("testdata is not a directory " + dir)
	}
	return dir
}()

func DumpDir(t *testing.T, tempDir string) {
	t.Helper()

	err := filepath.WalkDir(tempDir, func(dir string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		inf, err := d.Info()
		if err != nil {
			return fmt.Errorf("fs entry: %w", err)
		}
		t.Log(dir, inf.Name(), inf.Size(), inf.Mode())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func accessViolation(t *testing.T) bool {
	t.Helper()
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

type dat struct {
	name  string
	bytes int64
	crc32 uint32
}

const (
	TestDat1 = "TESTDAT1.TXT"
	TestDat2 = "TESTDAT2.TXT"
	TestDat3 = "TESTDAT3.TXT"
)

var texts = [3]dat{ //nolint:gochecknoglobals
	{TestDat1, 2009, 0xc71a6911},
	{TestDat2, 469, 0xc8ddc359},
	{TestDat3, 81_410, 0xc065e9c4},
}

func textnames(t *testing.T) [3]string {
	t.Helper()
	return [3]string{
		texts[0].name,
		texts[1].name,
		texts[2].name,
	}
}

// testingText confirms files match textnames.
func testingTexts(t *testing.T, files ...string) {
	t.Helper()
	for _, s := range textnames(t) {
		be.True(t, slices.Contains(files, s))
	}
}

// testingXTexts confirms the temp directory contains textnames.
// Matching both the file names and the file sizes.
func testingXTexts(t *testing.T, tempDir string) {
	t.Helper()

	names := textnames(t)
	config{
		root:         tempDir,
		wants:        3,
		enforceNames: true,
	}.extractor(t, names[:], texts[:])
}

func lowernames(t *testing.T) [3]string {
	t.Helper()
	return [3]string{
		strings.ToLower(texts[0].name),
		strings.ToLower(texts[1].name),
		strings.ToLower(texts[2].name),
	}
}

// testingLower confirms files match textnames using lowercase.
func testingLower(t *testing.T, files ...string) {
	t.Helper()
	for _, s := range lowernames(t) {
		be.True(t, slices.Contains(files, s))
	}
}

// testingXLower confirms the temp directory contains textnames.
// Matching both the file names and the file sizes.
func testingXLower(t *testing.T, tempDir string) {
	t.Helper()

	names := lowernames(t)
	config{
		root:         tempDir,
		wants:        3,
		enforceNames: true,
	}.extractor(t, names[:], texts[:])
}

var mixes = [15]dat{ //nolint:gochecknoglobals
	{"TEST.ANS", 68, 0x5ce2f707},
	{"TEST.ASC", 13, 0x7ef91c5d},
	{"TEST.BMP", 750_054, 0x64c6e850},
	{"TEST.CAP", 13, 0x49dfe6f2},
	{"TEST.DIZ", 13, 0xd16e82ea},
	{"TEST.DOC", 13, 0x9667e483},
	{"TEST.EXE", 2_426_368, 0x68e32163},
	{"TEST.GIF", 2_646, 0x195a28a8},
	{"TEST.JPG", 16_461, 0x134961cd},
	{"TEST.ME", 12, 0x3a3f1a1c},
	{"TEST.NFO", 13, 0xe5fd7de3},
	{"TEST.PCX", 29_530, 0xe63058f8},
	{"TEST.PNG", 4_163, 0xdfafce04},
	{"TEST.TXT", 14, 0xb8d80e3c},
	{"TEST~1.JPE", 16461, 0x134961cd},
}

func mixnames(t *testing.T) [15]string {
	t.Helper()
	return [15]string{
		mixes[0].name,
		mixes[1].name,
		mixes[2].name,
		mixes[3].name,
		mixes[4].name,
		mixes[5].name,
		mixes[6].name,
		mixes[7].name,
		mixes[8].name,
		mixes[9].name,
		mixes[10].name,
		mixes[11].name,
		mixes[12].name,
		mixes[13].name,
		mixes[14].name,
	}
}

// testingMixes confirms files match mixnames.
func testingMixes(t *testing.T, files ...string) {
	t.Helper()
	for _, s := range mixnames(t) {
		be.True(t, slices.Contains(files, s))
	}
}

func testingXMixes(t *testing.T, tempDir string) {
	t.Helper()

	names := mixnames(t)
	config{
		root:         tempDir,
		wants:        15,
		enforceNames: false,
	}.extractor(t, names[:], mixes[:])
}

type config struct {
	root         string // root directory to walk, should be t.TempDir()
	wants        int    // the number of expected files
	enforceNames bool   // throw errors for any unexpected extracted files
}

func (c config) extractor(t *testing.T, names []string, data []dat) {
	t.Helper()

	count := 0

	err := filepath.WalkDir(c.root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("test extracted walker: %w", err)
		}
		if info.IsDir() {
			return nil
		}
		find := slices.Index(names, info.Name())
		t.Log("extractor result>", info.Name(), info.Size())
		if c.enforceNames {
			be.True(t, find >= 0)
		}
		if find >= 0 && find < c.wants {
			name := data[find].name
			bytes := data[find].bytes
			t.Log("testing expecting", name, bytes)
			be.Equal(t, info.Size(), bytes)
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatal(c.root, err)
	}
	be.Equal(t, count, c.wants)
}

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

func Tests(t *testing.T) []TestData { //nolint:funlen
	tests := []TestData{
		{
			WantErr:  false,
			Testname: "7-Zip",
			Filename: Test7z, Ext: ".7z",
			cmdDos: "P7ZIP.EXE", cmdInfo: "p7zip, February 2009", cmdVersion: "4.65",
		},
		{
			WantErr:  false,
			Testname: "ARC",
			Filename: TestArc1, Ext: ".arc",
			cmdDos: "ARcnt.EXE", cmdInfo: "SEA ARC, January 1989", cmdVersion: "6.01",
		},
		{
			WantErr:  false,
			Testname: "ARJ",
			Filename: TestArj2, Ext: ".arj",
			cmdDos: "ARJ.EXE", cmdInfo: "Robert K Jung, December 1990", cmdVersion: "0.20 BETA",
		},
		{
			WantErr:  false,
			Testname: "ARJ with no extension",
			Filename: TestArjX, Ext: ".arj",
			cmdDos: "ARJ.EXE", cmdInfo: "Robert K Jung, December 1990", cmdVersion: "0.20 BETA",
		},
		{
			WantErr:  false,
			Testname: "BSD Tar",
			Filename: TestTar1, Ext: ".tar",
			cmdDos: "bsdtar", cmdInfo: "bsdtar", cmdVersion: "3.7.4",
		},
		{
			WantErr:  false,
			Testname: "Bzip2 Tar",
			Filename: TestBz2, Ext: ".tar.bz2",
			cmdDos: "bzip2", cmdInfo: "bzip2", cmdVersion: "1.0.8",
		},
		{
			WantErr:  false,
			Testname: "Microsoft Cabinet",
			Filename: TestCab, Ext: ".cab",
			cmdDos: "gcab", cmdInfo: "Microsoft Cabinet using Linux gcab", cmdVersion: "1.6",
		},
		{
			WantErr:  false,
			Testname: "Gzip BSD Tar",
			Filename: TestGzip1, Ext: ".tgz",
			cmdDos: "bsdtar", cmdInfo: "bsdtar", cmdVersion: "3.7.4",
		},
		{
			WantErr:  false,
			Testname: "Gzip",
			Filename: TestGzip2, Ext: gzx,
			cmdDos: "gzip", cmdInfo: "Free Software Foundation, 2023", cmdVersion: "1.13",
		},
		{
			WantErr:  false,
			Testname: "LHA/LZH",
			Filename: TestLZH2, Ext: ".lha",
			cmdDos: "LHARcnt.EXE", cmdInfo: "LHarc, May 1990", cmdVersion: "1.13",
		},
		{
			WantErr:  false,
			Testname: "LHA/LH0",
			Filename: TestLZH1, Ext: ".lha",
			cmdDos: "LHARcnt.EXE", cmdInfo: "LHarc, May 1990", cmdVersion: "1.13",
		},
		{
			WantErr:  false,
			Testname: "LHA/LH5",
			Filename: TestLZH3, Ext: ".lha",
			cmdDos: "LHARcnt.EXE", cmdInfo: "LHarc, May 1990", cmdVersion: "1.13",
		},
		{
			WantErr:  false,
			Testname: "RAR",
			Filename: TestRar1, Ext: ".rar",
			cmdDos: "RAR.EXE", cmdInfo: "RAR archiver, 1999", cmdVersion: "2.50",
		},
		{
			WantErr:  false,
			Testname: "XZ Utils Tar",
			Filename: TestXZ, Ext: ".tar.xz",
			cmdDos: "xz", cmdInfo: "XZ Utils", cmdVersion: "5.6.2",
		},
		{
			WantErr:  false,
			Testname: "Zstandard Tar",
			Filename: TestZstd, Ext: ".tar.zst",
			cmdDos: "zstd", cmdInfo: "Zstandard by Yann Collet", cmdVersion: "1.5.6",
		},
		{
			WantErr:  false,
			Testname: "Implode ZIP",
			Filename: TestImpode, Ext: ".zip",
			cmdDos: "hwzip", cmdInfo: "Impode", cmdVersion: "2.3",
		},
		{
			WantErr:  false,
			Testname: "Reduce ZIP",
			Filename: TestReduce, Ext: ".zip",
			cmdDos: "hwzip", cmdInfo: "Reduce", cmdVersion: "2.3",
		},
		{
			WantErr:  false,
			Testname: "Shrink ZIP",
			Filename: TestShrink, Ext: ".zip",
			cmdDos: "hwzip", cmdInfo: "Shrink", cmdVersion: "2.3",
		},
		{
			WantErr:  false,
			Testname: "Pak",
			Filename: "PAK100.PAK", Ext: ".pak",
			cmdDos: "PAK.EXE", cmdInfo: "NoGate Consulting, 1988", cmdVersion: "1.0",
		},
		{
			WantErr:  false,
			Testname: "XZ",
			Filename: "TEST.EXE.xz", Ext: ".xz",
			cmdDos: "xz", cmdInfo: "XZ Utils", cmdVersion: "5.8.3",
		},
		{
			WantErr:  false,
			Testname: "BZip2",
			Filename: "TEST.EXE.bz2", Ext: ".bz2",
			cmdDos: "bzip2", cmdInfo: "block-sorting compressor", cmdVersion: "1.0.8",
		},
		{
			WantErr:  false,
			Testname: "Zstandard",
			Filename: "TEST.EXE.zst", Ext: ".zst",
			cmdDos: "zstd", cmdInfo: "Zstandard by Yann Collet", cmdVersion: "1.5.7",
		},
		{
			WantErr:  true,
			Testname: "Not an archive",
			Filename: TestDat1, Ext: ".txt",
			cmdDos: "", cmdInfo: "", cmdVersion: "",
		},
	}
	if accessViolation(t) {
		tests = slices.DeleteFunc(tests, func(tt TestData) bool {
			return tt.cmdDos == "ARJ.EXE" // the arj tool on macOS now gets killed by the system
		})
	}
	return tests
}

func TestData_ReadContent(t *testing.T) {
	t.Parallel()

	for _, tt := range Tests(t) {
		const wantThree, wantOne = 3, 1
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			t.Log("Archive Read content for", tt.Testname)

			var c archive.Content
			src := filepath.Join(testdata, tt.Filename)
			err := c.Read(t.Context(), src)
			if tt.WantErr || tt.Ext == ".zst" {
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)
			got := len(c.Files)
			switch tt.Ext {
			case ".bz2", gzx, ".xz":
				be.Equal(t, got, wantOne)
			default:
				be.Equal(t, got, wantThree)
			}
		})
	}
}

func TestData_Extract(t *testing.T) {
	t.Parallel()

	for _, tt := range Tests(t) {
		const wantThree, wantOne = 3, 1
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			t.Log("Archive Extract content for", tt.Testname)

			tmp := t.TempDir()
			err := archive.Extractor{
				Source:      filepath.Join(testdata, tt.Filename),
				Destination: tmp,
			}.Extract(t.Context())
			if tt.WantErr {
				t.Log("Expected an error, got", err)
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)
			extracted, err := helper.Count(tmp)
			be.Err(t, err, nil)
			switch tt.Ext {
			case gzx:
				be.Equal(t, extracted, wantOne)
				testdataExtractGzip(t, tmp)
			case ".bz2", ".xz", ".zst":
				be.Equal(t, extracted, wantOne)
			default:
				be.Equal(t, extracted, wantThree)
			}
		})
	}
}

func testdataExtractGzip(t *testing.T, tmp string) {
	t.Helper()
	t.Log("Gzip test archive only contains a single test file")

	items, err := os.ReadDir(tmp)
	be.Err(t, err, nil)

	const wantOne = 1
	count := len(items)
	be.Equal(t, count, wantOne)
	if count < wantOne {
		return
	}
	be.Equal(t, items[0].Name(), TestDat3)
	info, err := items[0].Info()
	be.Err(t, err, nil)
	be.True(t, !info.IsDir())
	const wantSize = int64(81410)
	be.Equal(t, info.Size(), wantSize)
}

func TestData_Extract_WithTargets(t *testing.T) {
	t.Parallel()

	for _, tt := range Tests(t) {
		const want2Targets = 2

		// unsupported returns true for tools or packages that don't extract file targets
		unsupported := func() bool {
			switch tt.Ext {
			case gzx, ".bz2", ".cab", ".xz", ".zst":
				return true
			}
			// while the zipfile handlers do support targets,
			// the hwzip tool for legacy archives does not.
			switch tt.Filename {
			case TestShrink, TestReduce:
				return true
			}
			return false
		}

		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()

			if unsupported() {
				t.Log("skipped unsupported testcase", tt.Testname)
				return
			}

			tmp := t.TempDir()
			err := archive.Extractor{
				Source:      filepath.Join(testdata, tt.Filename),
				Destination: tmp,
			}.Extract(t.Context(), TestDat2, TestDat3)
			if tt.WantErr {
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)

			extracted, err := helper.Count(tmp)
			be.Err(t, err, nil)
			be.Equal(t, extracted, want2Targets)
		})
	}
}

func TestData_Extract_Zips(t *testing.T) {
	t.Parallel()

	for _, tt := range Tests(t) {
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()

			const pkzip = ".zip"
			if tt.Ext != pkzip {
				return
			}
			src := filepath.Join(testdata, tt.Filename)
			tmp := t.TempDir()

			err := archive.Extractor{
				Source:      src,
				Destination: tmp,
			}.Zips(t.Context())
			be.Err(t, err, nil)

			err = archive.Extractor{
				Source:      src,
				Destination: tmp,
			}.Zips(t.Context(), TestDat2, TestDat3)
			switch tt.Filename {
			case TestShrink, TestReduce, TestImpode:
				// skip these for now
				return
			default:
				const want2Targets = 2
				be.Err(t, err, nil)
				extracted, err := helper.Count(tmp)
				be.Err(t, err, nil)
				be.Equal(t, extracted, want2Targets)
			}
		})
	}
}

func TestData_ExtractTemp(t *testing.T) {
	t.Parallel()

	for _, tt := range Tests(t) {
		t.Run(tt.Testname, func(t *testing.T) {
			t.Parallel()
			if ok := tt.Ext != ".txt"; !ok {
				return
			}
			t.Log("Specialized source extraction on", tt.Testname)

			src := filepath.Join(testdata, tt.Filename)
			got, err := archive.ExtractTemp(t.Context(), src)
			if tt.WantErr {
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
	for _, tt := range Tests(t) {
		t.Run(tt.Testname, func(t *testing.T) {
			t.Log("Never run TestList in parallel, it will cause failures")

			ctx := t.Context()
			src := filepath.Join(testdata, tt.Filename)
			got, err := archive.List(ctx, src, tt.Filename)
			if tt.WantErr {
				be.Err(t, err)
				return
			}
			be.Err(t, err, nil)
			notEmpty := len(got) > 0
			be.True(t, notEmpty)
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
			".arj", false,
		},
		{
			"Missing TAR GZ extension", ".tar.gz", "ARCHIVE",
			".tar.gz", false,
		},
		{
			"Not a valid extension", "arj", "ARCHIVE",
			".arj", true,
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
			defer os.Remove(got)
			be.True(t, strings.HasSuffix(got, tt.want))
			be.True(t, strings.HasPrefix(filepath.Base(got), tt.src+"-"))
		})
	}
}
