package expand_test

import (
	"archive/zip"
	"fmt"
	"hash/crc32"
	"io"
	"path/filepath"

	"github.com/Defacto2/archive/expand"
)

// Example demonstrates the basic usage of the package.
func Example() {
	const testdata = "testdata"
	const testFile = "example-r.zip"

	// Register the Expand decompressors with the archive zip package.
	expand.Register()

	name := filepath.Join(testdata, testFile)
	rc, _ := zip.OpenReader(name)
	defer rc.Close()

	for n, f := range rc.File {
		fr, _ := f.Open()
		hasher := crc32.NewIEEE()
		size := int64(f.UncompressedSize64) //nolint:gosec
		_, _ = io.CopyN(hasher, fr, size)

		fmt.Printf("%d. Decompressed method %d: %s, %dB\n"+
			"Stored zip crc32 %x, and copied to buffer: %x\n",
			n+1, f.Method, f.Name, f.UncompressedSize64, f.CRC32, hasher.Sum32())

		fr.Close()
	}
	// Output: 1. Decompressed method 5: R.BIN, 2048B
	// Stored zip crc32 8314ddd9, and copied to buffer: 8314ddd9
}
