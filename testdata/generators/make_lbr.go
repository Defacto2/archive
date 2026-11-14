package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	sectorSize = 128
	entrySize  = 32
	padByte    = 0x1A
)

// crc16CCITT calculates the XMODEM/CCITT CRC-16 (poly 0x1021, init 0x0000).
func crc16CCITT(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if (crc & 0x8000) != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func padName(s string, width int) []byte {
	out := bytes.Repeat([]byte{' '}, width)
	copy(out, strings.ToUpper(s))
	return out
}

func main() {
	sources := []string{
		"/home/ben/github/archive/testdata/TESTDAT1.TXT",
		"/home/ben/github/archive/testdata/TESTDAT2.TXT",
		"/home/ben/github/archive/testdata/TESTDAT3.TXT",
	}

	totalEntries := 1 + len(sources) // Entry 0 (directory) + members
	dirSectors := (totalEntries*entrySize + sectorSize - 1) / sectorSize
	dirBuf := make([]byte, dirSectors*sectorSize)

	// Entry 0: Directory descriptor
	dirBuf[0] = 0x00
	copy(dirBuf[1:9], padName("", 8))
	copy(dirBuf[9:12], padName("", 3))
	binary.LittleEndian.PutUint16(dirBuf[12:14], 0)
	binary.LittleEndian.PutUint16(dirBuf[14:16], uint16(dirSectors))

	var dataBuf bytes.Buffer
	curSector := uint16(dirSectors)

	for i, src := range sources {
		content, err := os.ReadFile(src)
		if err != nil {
			panic(err)
		}

		base := filepath.Base(src)
		ext := strings.TrimPrefix(filepath.Ext(base), ".")
		name := strings.TrimSuffix(base, filepath.Ext(base))

		sectors := uint16((len(content) + sectorSize - 1) / sectorSize)
		padCount := byte((sectorSize - (len(content) % sectorSize)) % sectorSize)

		// Pad sector data with 0x1A per LUDEF5
		var memberData bytes.Buffer
		memberData.Write(content)
		if padCount > 0 {
			memberData.Write(bytes.Repeat([]byte{padByte}, int(padCount)))
		}

		crc := crc16CCITT(content)

		off := (i + 1) * entrySize
		dirBuf[off] = 0x00
		copy(dirBuf[off+1:off+9], padName(name, 8))
		copy(dirBuf[off+9:off+12], padName(ext, 3))
		binary.LittleEndian.PutUint16(dirBuf[off+12:off+14], curSector)
		binary.LittleEndian.PutUint16(dirBuf[off+14:off+16], sectors)
		binary.LittleEndian.PutUint16(dirBuf[off+16:off+18], crc)
		dirBuf[off+26] = padCount

		dataBuf.Write(memberData.Bytes())
		curSector += sectors
		fmt.Printf("Added %s: sector=%d, sectors=%d, pad=%d, size=%d, crc=0x%04X\n",
			base, curSector-sectors, sectors, padCount, len(content), crc)
	}

	var out bytes.Buffer
	out.Write(dirBuf)
	out.Write(dataBuf.Bytes())

	dst := "/home/ben/github/archive/testdata/TEST.LBR"
	if err := os.WriteFile(dst, out.Bytes(), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %s (%d bytes, %d sectors)\n", dst, out.Len(), curSector)
}
