package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const (
	arcMark = 0x1A
	dle     = 0x90
)

func crc16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if (crc & 1) != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// Encode RLE (Non-Repeat Compression / Method 3)
func packRLE(data []byte) []byte {
	var out bytes.Buffer
	n := len(data)
	i := 0
	for i < n {
		b := data[i]
		runLen := 1
		for i+runLen < n && data[i+runLen] == b && runLen < 255 {
			runLen++
		}

		if b == dle {
			// Literal DLE is encoded as 0x90 0x00
			for k := 0; k < runLen; k++ {
				out.WriteByte(dle)
				out.WriteByte(0)
			}
		} else if runLen > 3 {
			// Run of 4 or more: <char> 0x90 <count>
			out.WriteByte(b)
			out.WriteByte(dle)
			out.WriteByte(byte(runLen))
		} else {
			for k := 0; k < runLen; k++ {
				out.WriteByte(b)
			}
		}
		i += runLen
	}
	return out.Bytes()
}

func main() {
	testfile := "/home/ben/github/archive/testdata/TESTDAT1.TXT"
	content, err := os.ReadFile(testfile)
	if err != nil {
		panic(err)
	}

	packed := packRLE(content)
	crc := crc16(content)

	var arcBuf bytes.Buffer
	// Header: 1a 03
	arcBuf.WriteByte(arcMark)
	arcBuf.WriteByte(3) // Method 3: Packed

	// 13 bytes filename null-padded
	name := filepath.Base(testfile)
	var nameBytes [13]byte
	copy(nameBytes[:], name)
	arcBuf.Write(nameBytes[:])

	// 4 bytes compressed size
	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], uint32(len(packed)))
	arcBuf.Write(b4[:])

	// 2 bytes date, 2 bytes time
	arcBuf.Write([]byte{0x21, 0x14, 0x48, 0x82}) // Feb 14 2025

	// 2 bytes CRC
	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[:], crc)
	arcBuf.Write(b2[:])

	// 4 bytes uncompressed size
	binary.LittleEndian.PutUint32(b4[:], uint32(len(content)))
	arcBuf.Write(b4[:])

	// Data
	arcBuf.Write(packed)

	// End of archive marker: 1a 00
	arcBuf.Write([]byte{arcMark, 0})

	dst := "/home/ben/github/archive/testdata/ARC_PACK.ARC"
	err = os.WriteFile(dst, arcBuf.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %s (orig=%d, packed=%d)\n", dst, len(content), len(packed))
}
