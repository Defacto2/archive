package archive_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Defacto2/archive"
	"github.com/Defacto2/archive/rezip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ExampleReadme() {
	name := archive.Readme("APP.ZIP", "APP.EXE", "APP.TXT", "APP.BIN", "APP.DAT", "STUFF.DAT")
	fmt.Println(name)
	// Output: APP.TXT
}

func TestUsage(t *testing.T) {
	t.Parallel()

	zipfile := "testdata/PKZ80A1.ZIP"
	zipfalse := "testdata/test.zip"
	dst := filepath.Join(os.TempDir(), "archive_test")

	err := archive.ExtractAll(zipfalse, os.TempDir())
	require.Error(t, err)
	err = archive.ExtractAll(zipfile, dst)
	defer os.RemoveAll(dst)
	require.NoError(t, err)

	x := archive.Extractor{
		Source:      zipfalse,
		Destination: dst,
	}
	err = x.Extract()
	require.Error(t, err)

	x = archive.Extractor{
		Source:      zipfile,
		Destination: dst,
	}
	err = x.Extract()
	defer os.RemoveAll(dst)
	require.NoError(t, err)

	path, err := archive.ExtractSource(zipfalse, "archive_test")
	require.Error(t, err)
	require.Empty(t, path)

	path, err = archive.ExtractSource(zipfile, "")
	defer os.RemoveAll(path)
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	files, err := archive.List(zipfalse, "test.zip")
	require.Error(t, err)
	require.Empty(t, files)

	files, err = archive.List(zipfile, "PKZ80A1.ZIP")
	require.NoError(t, err)
	assert.Len(t, files, 15)

	name := archive.Readme(zipfalse, "test.zip")
	require.Empty(t, name)

	name = archive.Readme(zipfile, "PKZ80A1.ZIP")
	assert.Empty(t, name)

	_, err = rezip.Compress(zipfalse, dst)
	require.Error(t, err)

	toComp := "testdata/TEST.EXE"
	dstComp := filepath.Join(os.TempDir(), "archive_test.zip")
	_ = os.Remove(dstComp)

	_, err = rezip.Compress(toComp, dstComp)
	require.NoError(t, err)
	_, err = rezip.Compress(toComp, dstComp)
	require.Error(t, err)
	_ = os.Remove(dstComp)

	_, err = rezip.CompressDir(zipfalse, dstComp)
	require.Error(t, err)
	_ = os.Remove(dstComp)
	_, err = rezip.CompressDir("testdata", dstComp)
	require.NoError(t, err)
	_, err = rezip.CompressDir("testdata", dstComp)
	require.Error(t, err)
	_ = os.Remove(dstComp)
}
