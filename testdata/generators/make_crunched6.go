package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const (
	arcMark    = 0x1A
	oldTabSize = 4096
	noPred     = 0xFFFF
	dle        = 0x90
)

type stringEntry struct {
	used        bool
	follower    byte
	next        uint16
	predecessor uint16
}

func oldHash(pred uint16, foll byte) uint16 {
	local := (uint32(pred+uint16(foll)) | 0x0800) & 0xFFFF
	local *= local
	return uint16((local >> 6) & 0x0FFF)
}

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
			for k := 0; k < runLen; k++ {
				out.WriteByte(dle)
				out.WriteByte(0)
			}
		} else if runLen > 3 {
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

func crunch5(data []byte) []byte {
	stringTab := make([]stringEntry, oldTabSize)

	eolist := func(index uint16) uint16 {
		for stringTab[index].next != 0 {
			index = stringTab[index].next
		}
		return index
	}

	hashAdd := func(pred uint16, foll byte) uint16 {
		local := oldHash(pred, foll)
		if !stringTab[local].used {
			return local
		}

		last := eolist(local)
		next := (last + 101) & 0x0FFF
		for stringTab[next].used {
			next = (next + 1) & 0x0FFF
		}
		stringTab[last].next = next
		return next
	}

	find := func(pred uint16, foll byte) (uint16, bool) {
		local := oldHash(pred, foll)
		if !stringTab[local].used {
			return 0, false
		}
		curr := local
		for {
			if stringTab[curr].predecessor == pred && stringTab[curr].follower == foll {
				return curr, true
			}
			if stringTab[curr].next == 0 {
				break
			}
			curr = stringTab[curr].next
		}
		return 0, false
	}

	updTab := func(pred uint16, foll byte) {
		idx := hashAdd(pred, foll)
		stringTab[idx] = stringEntry{
			used:        true,
			follower:    foll,
			next:        0,
			predecessor: pred,
		}
	}

	// Initialize 256 single-byte roots
	for i := 0; i < 256; i++ {
		updTab(noPred, byte(i))
	}
	codeCount := oldTabSize - 256

	var codes []int
	pred := uint16(noPred)

	for _, b := range data {
		if pred == noPred {
			idx, _ := find(noPred, b)
			pred = idx
			continue
		}

		code, found := find(pred, b)
		if found {
			pred = code
		} else {
			codes = append(codes, int(pred))
			if codeCount > 0 {
				updTab(pred, b)
				codeCount--
			}
			idx, _ := find(noPred, b)
			pred = idx
		}
	}
	if pred != noPred {
		codes = append(codes, int(pred))
	}

	// Pack 12-bit codes into 3-byte pairs
	var out bytes.Buffer
	for i := 0; i < len(codes); i += 2 {
		c1 := codes[i]
		if i+1 < len(codes) {
			c2 := codes[i+1]
			b0 := byte(c1 >> 4)
			b1 := byte(((c1 & 0x0F) << 4) | ((c2 >> 8) & 0x0F))
			b2 := byte(c2 & 0xFF)
			out.WriteByte(b0)
			out.WriteByte(b1)
			out.WriteByte(b2)
		} else {
			b0 := byte(c1 >> 4)
			b1 := byte((c1 & 0x0F) << 4)
			out.WriteByte(b0)
			out.WriteByte(b1)
		}
	}
	return out.Bytes()
}

func main() {
	testfile := "/home/ben/github/archive/testdata/TESTDAT2.TXT"
	content, err := os.ReadFile(testfile)
	if err != nil {
		panic(err)
	}

	rled := packRLE(content)
	crunched := crunch5(rled)
	crc := crc16(content)

	var arcBuf bytes.Buffer
	arcBuf.WriteByte(arcMark)
	arcBuf.WriteByte(6) // Method 6: Old Crunched with RLE

	name := filepath.Base(testfile)
	var nameBytes [13]byte
	copy(nameBytes[:], name)
	arcBuf.Write(nameBytes[:])

	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], uint32(len(crunched)))
	arcBuf.Write(b4[:])

	arcBuf.Write([]byte{0x21, 0x14, 0x48, 0x82}) // Date/Time

	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[:], crc)
	arcBuf.Write(b2[:])

	binary.LittleEndian.PutUint32(b4[:], uint32(len(content)))
	arcBuf.Write(b4[:])

	arcBuf.Write(crunched)
	arcBuf.Write([]byte{arcMark, 0})

	dst := "/home/ben/github/archive/testdata/ARC_CRU6.ARC"
	err = os.WriteFile(dst, arcBuf.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %s with Method 6 (orig=%d, crunched=%d)\n", dst, len(content), len(crunched))
}
