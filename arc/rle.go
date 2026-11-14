package arc

import (
	"bytes"
)

const dle = 0x90 // Repeat byte marker

// UnpackRLE decompresses data compressed using SEA ARC Non-Repeat (NCR) / RLE90.
func UnpackRLE(data []byte) []byte {
	var buf bytes.Buffer
	var c byte

	repeat := false

	for _, b := range data {
		if repeat {
			if b == 0 {
				buf.WriteByte(dle)
			} else {
				for count := int(b) - 1; count > 0; count-- {
					buf.WriteByte(c)
				}
			}
			repeat = false
			continue
		}
		if b == dle {
			repeat = true
		} else {
			c = b
			buf.WriteByte(b)
		}
	}

	return buf.Bytes()
}
