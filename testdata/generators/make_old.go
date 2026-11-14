package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const arcMark = 0x1A

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

func main() {
	testfile := "/home/ben/github/archive/testdata/TESTDAT1.TXT"
	content, err := os.ReadFile(testfile)
	if err != nil {
		panic(err)
	}
	crc := crc16(content)

	var arcBuf bytes.Buffer
	// Header: 1a 01
	arcBuf.WriteByte(arcMark)
	arcBuf.WriteByte(1) // Method 1: Old Stored

	// 13 bytes filename null-padded
	name := filepath.Base(testfile)
	var nameBytes [13]byte
	copy(nameBytes[:], name)
	arcBuf.Write(nameBytes[:])

	// 4 bytes size
	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], uint32(len(content)))
	arcBuf.Write(b4[:])

	// 2 bytes date, 2 bytes time
	arcBuf.Write([]byte{0x21, 0x14, 0x48, 0x82}) // Feb 14 2025

	// 2 bytes CRC
	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[:], crc)
	arcBuf.Write(b2[:])

	// NOTICE: No uncompressed length field for Method 1! (23 bytes header)

	// Data
	arcBuf.Write(content)

	// End of archive marker
	arcBuf.Write([]byte{arcMark, 0})

	dst := "/home/ben/github/archive/testdata/ARC_OLD.ARC"
	err = os.WriteFile(dst, arcBuf.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %s with Method 1 (size=%d)\n", dst, len(content))
}
