package explode //nolint:testpackage

import (
	"bytes"
	"io"
	"testing"
)

func TestCorruptStream(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"incomplete_header", []byte{0x05, 0x12}},
		{"random_bytes", []byte{0x02, 0xFF, 0x00, 0xAA, 0x55}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newReader(bytes.NewReader(tc.data), 4096, 0)
			defer r.Close()

			buf := make([]byte, 1024)
			_, err := io.ReadFull(r, buf)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}
