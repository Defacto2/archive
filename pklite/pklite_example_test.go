package pklite_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Defacto2/archive/pklite"
)

// Example demonstrates unpacking a PKLITE compressed COM file.
func Example() {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "D2_V115.COM"))
	if err != nil {
		fmt.Println(err)
		return
	}
	out, err := pklite.Decompress(data)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%d bytes packed, %d bytes unpacked\n", len(data), len(out))
	// Output: 877 bytes packed, 4176 bytes unpacked
}
