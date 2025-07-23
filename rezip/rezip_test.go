package rezip_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Defacto2/archive/rezip"
	"github.com/nalgeon/be"
)

func td(name string) string {
	_, file, _, usable := runtime.Caller(0)
	if !usable {
		panic("runtime.Caller failed")
	}
	d := filepath.Join(filepath.Dir(file), "..")
	return filepath.Join(d, "testdata", name)
}

func TestCompress(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := td("TEST.EXE")
	dest := filepath.Join(tmp, "zip_test.zip")
	inf, err := os.Stat(src)
	be.Err(t, err, nil)
	size, err := rezip.Compress(src, dest)
	be.Err(t, err, nil)
	be.Equal(t, int64(size), inf.Size())
	// confirm the zip file is smaller than the total size of the files
	inf, err = os.Stat(dest)
	be.Err(t, err, nil)
	less := inf.Size() < int64(size)
	be.True(t, less)
	// confirm command fails when the file already exists
	size, err = rezip.Compress(src, dest)
	be.Err(t, err)
	be.Equal(t, size, 0)
	// confirm command fails when the dest is a directory
	size, err = rezip.Compress(src, tmp)
	be.Err(t, err)
	be.Equal(t, size, 0)
}

func TestCompressDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	srcDir := td("")
	dest := filepath.Join(tmp, "unzip_test.zip")
	size, err := rezip.CompressDir(srcDir, dest)
	be.Err(t, err, nil)
	const fourMB = 4 * 1024 * 1024
	greater := size > int64(fourMB)
	be.True(t, greater)
	// confirm the zip file is smaller than the total size of the files
	inf, err := os.Stat(dest)
	be.Err(t, err, nil)
	less := inf.Size() < size
	be.True(t, less)
}

func TestUnzip(t *testing.T) {
	t.Parallel()
	src := td("PKZ80A1.ZIP")
	err := rezip.Test(src)
	be.Err(t, err, nil)
	src = td("ARJ310.ARJ")
	err = rezip.Test(src)
	be.Err(t, err)
}
