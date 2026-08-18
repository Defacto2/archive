//go:build !(darwin && arm64)

package archive_test

import "testing"

func init(t *testing.T) {
	t.Helper()

	t.Log("arj support has been disabled for macOS on Apple Silicon")
}
