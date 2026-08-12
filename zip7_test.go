package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/nalgeon/be"
)

const (
	Test7z = "7ZIP465.7Z"
)

func Test7zContent(t *testing.T) {
	t.Parallel()

	name1 := filepath.Join(testdata, Test7z)
	var c archive.Content
	err := c.Zip7(t.Context(), name1)
	be.Err(t, err, nil)

	count := len(c.Files)
	const want = 3
	be.Equal(t, c.Ext, ".7z")
	be.Equal(t, count, want)
	testingTexts(t, c.Files...)
}

func Test7zExtractor(t *testing.T) {
	t.Parallel()

	name1 := filepath.Join(testdata, Test7z)
	x := archive.Extractor{
		Source:      name1,
		Destination: t.TempDir(),
	}
	err := x.Zip7(t.Context())
	be.Err(t, err, nil)
	testingXTexts(t, x.Destination)
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

	files := archive.Zip7s([]byte(output1))
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
	files = archive.Zip7s([]byte(output2))
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
	files = archive.Zip7s([]byte(output3))
	be.Equal(t, len(files), 0)
}
