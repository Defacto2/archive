package archive

import (
	"testing"

	"github.com/nalgeon/be"
)

// File archive_internal_test.go is exclusive for private archive funcs
// that require testing but don't need to be in the package API.

func TestArcs(t *testing.T) {
	t.Parallel()

	const output1 = `
Name          Length    Date
============  ========  =========
TESTDAT1.TXT      2009  14 Feb 25
README             469  14 Feb 25
TESTDAT3.TXT     81410  14 Feb 25
          ====  ========
Total        3     83888
`

	files := arcs([]byte(output1))
	count := len(files)

	const want = 3
	be.Equal(t, count, want)
	if count >= want {
		be.Equal(t, files[0], "TESTDAT1.TXT")
		be.Equal(t, files[1], "README")
		be.Equal(t, files[2], "TESTDAT3.TXT")
	}
}

func TestArjFiles(t *testing.T) {
	t.Parallel()

	const output1 = `
ARJ 2.78 Copyright (c) 1998-2004 Robert K Jung

Filename       Original Compressed Ratio DateTime modified Attributes/GUA BPMGS
------------ ---------- ---------- ----- ----------------- -------------- -----
TESTDAT1.TXT       2009        889 0.443 25-02-14 13:21:10                  1
TESTDAT2.TXT        469        266 0.567 25-02-14 13:17:34                  1
TESTDAT3.TXT      81410      22438 0.276 25-02-14 13:21:02                  1
------------ ---------- ---------- -----
      3 files      83888      23593 0.281
`

	files := arjs([]byte(output1))
	count := len(files)

	const want = 3
	be.Equal(t, count, want)
	if count >= want {
		be.Equal(t, files[0], "TESTDAT1.TXT")
		be.Equal(t, files[1], "TESTDAT2.TXT")
		be.Equal(t, files[2], "TESTDAT3.TXT")
	}

	const output2 = `
	 Filename       Original Compressed Ratio DateTime modified Attributes/GUA BPMGS
	 ------------ ---------- ---------- ----- ----------------- -------------- -----
	 TESTDAT1.TXT       2009        889 0.443 25-02-14 13:21:10                  1
	 TESTDAT2.TXT        469        266 0.567 25-02-14 13:17:34                  1
	 TESTDAT3.TXT      81410      22438 0.276 25-02-14 13:21:02                  1
	 ------------ ---------- ---------- -----
	      3 files      83888      23593 0.281
		`

	files = arjs([]byte(output2))
	count = len(files)

	be.Equal(t, count, want)
	if count >= want {
		be.Equal(t, files[0], "TESTDAT1.TXT")
		be.Equal(t, files[1], "TESTDAT2.TXT")
		be.Equal(t, files[2], "TESTDAT3.TXT")
	}
}

func TestGzipName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.txt.gz", "example.txt"},
		{"archive.tar.gz", "archive.tar"},
		{"/path/to/data.csv.gz", "data.csv"},
		{"file_without_ext.gz", "file_without_ext"},
		{"noextension", "noextension"},
		{".hidden.gz", ".hidden"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := gzipName(tt.input); got != tt.expected {
				t.Errorf("gzipName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLHAFiles(t *testing.T) {
	t.Parallel()

	const output1 = `
PERMSSN    UID  GID      SIZE  RATIO      STAMP            NAME
---------- ----------- ------- ------ ------------ --------------------
[generic]                 2009  48.8% Feb 14 13:21 testdat1.txt
[generic]                  469  66.5% Feb 14 13:17 testdat2.txt
[generic]                81410  29.5% Feb 14 13:21 testdat3.txt
---------- ----------- ------- ------ ------------ --------------------
 Total         3 files   83888  30.2% Feb 14 07:19
`

	files := lhas([]byte(output1))
	count := len(files)

	const want = 3
	be.Equal(t, count, want)
	if count >= want {
		be.Equal(t, files[0], "testdat1.txt")
		be.Equal(t, files[1], "testdat2.txt")
		be.Equal(t, files[2], "testdat3.txt")
	}

	const output2 = `
PERMSSN    UID  GID      SIZE  RATIO      STAMP            NAME
---------- ----------- ------- ------ ------------ --------------------
[generic]                 1024  50.0% Feb 14 13:21 docs/my report.pdf
[generic]                 2048  40.0% Feb 14 13:22 images/photo.png
---------- ----------- ------- ------ ------------ --------------------
 Total         2 files    3072  43.3% Feb 14 13:22
		`

	files = lhas([]byte(output2))
	count = len(files)

	const want2 = 2
	be.Equal(t, count, want2)
	if count >= want2 {
		be.Equal(t, files[0], "docs/my report.pdf")
		be.Equal(t, files[1], "images/photo.png")
	}
}

func TestZip7Files(t *testing.T) {
	t.Parallel()

	const output1 = `
7-Zip 23.01 (x64) : Copyright (c) 1999-2023 Igor Pavlov : 2023-06-20

Scanning the drive for archives:
1 file, 83888 bytes (82 KiB)

Listing archive: test.7z

--
Path = test.7z
Type = 7z
Physical Size = 83888

   Date      Time    Attr         Size   Compressed  Name
------------------- ----- ------------ ------------  ------------------------
2025-02-15 00:21:10 ....A         2009        20465  TESTDAT1.TXT
2025-02-15 00:17:34 ....A          469               folder/TESTDAT2.TXT
2025-02-15 00:21:02 D....            0            0  folder
------------------- ----- ------------ ------------  ------------------------
2025-02-15 00:21:10              83888        20465  2 files, 1 directory
`

	files := zip7s([]byte(output1))
	count := len(files)

	const want = 2
	be.Equal(t, count, want)
	if count >= want {
		be.Equal(t, files[0], "TESTDAT1.TXT")
		be.Equal(t, files[1], "folder/TESTDAT2.TXT")
	}

	const output2 = `

	    Date      Time    Attr         Size   Compressed  Name
	 ------------------- ----- ------------ ------------  ------------------------
	 2025-02-15 00:21:10 ....A         2009        20465  TESTDAT1.TXT
	 2025-02-15 00:17:34 ....A          469               TESTDAT2.TXT
	 2025-02-15 00:21:02 ....A        81410               TESTDAT3.TXT
	 ------------------- ----- ------------ ------------  ------------------------
	 2025-02-15 00:21:10              83888        20465  3 files
`
	files = zip7s([]byte(output2))
	count = len(files)

	const want2 = 3
	be.Equal(t, count, want2)
	if count >= want2 {
		be.Equal(t, files[0], "TESTDAT1.TXT")
		be.Equal(t, files[1], "TESTDAT2.TXT")
		be.Equal(t, files[2], "TESTDAT3.TXT")
	}

	const output3 = `
		7-Zip 23.01 (x64) : Copyright (c) 1999-2023 Igor Pavlov : 2023-06-20
`
	files = zip7s([]byte(output3))
	be.Equal(t, len(files), 0)
}

func Test_zipInfos(t *testing.T) {
	t.Parallel()

	input := []byte("file1.txt\r\nfile2.jpg\n\nsub/file3.go\r\n")
	got := zipInfos(input)

	want := []string{"file1.txt", "file2.jpg", "sub/file3.go"}
	be.Equal(t, got, want)
}
