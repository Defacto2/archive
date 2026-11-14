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

func williamsHash(pred uint16, foll byte) uint16 {
	return uint16(((uint32(pred) + uint32(foll)) * 15073) & 0x0FFF)
}

func crc16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
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
			for range runLen {
				out.WriteByte(dle)
				out.WriteByte(0)
			}
		} else if runLen > 3 {
			out.WriteByte(b)
			out.WriteByte(dle)
			out.WriteByte(byte(runLen))
		} else {
			for range runLen {
				out.WriteByte(b)
			}
		}
		i += runLen
	}
	return out.Bytes()
}

func crunch7(data []byte) []byte {
	stringTab := make([]stringEntry, oldTabSize)

	eolist := func(index uint16) uint16 {
		for stringTab[index].next != 0 {
			index = stringTab[index].next
		}
		return index
	}

	hashAdd := func(pred uint16, foll byte) uint16 {
		local := williamsHash(pred, foll)
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
		local := williamsHash(pred, foll)
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

	for i := range 256 {
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

	// Pack 12-bit codes big-endian style: 2 codes per 3 bytes
	var out bytes.Buffer
	for i := 0; i < len(codes); i += 2 {
		c1 := codes[i]
		if i+1 < len(codes) {
			c2 := codes[i+1]
			out.WriteByte(byte(c1 >> 4))
			out.WriteByte(byte(((c1 & 0x0F) << 4) | (c2 >> 8)))
			out.WriteByte(byte(c2 & 0xFF))
		} else {
			out.WriteByte(byte(c1 >> 4))
			out.WriteByte(byte((c1 & 0x0F) << 4))
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
	crc := crc16(content)
	rleData := packRLE(content)
	crunched := crunch7(rleData)

	var arcBuf bytes.Buffer
	arcBuf.WriteByte(arcMark)
	arcBuf.WriteByte(7) // Method 7: Crunched with Ross Williams' hash + RLE

	name := filepath.Base(testfile)
	var nameBytes [13]byte
	copy(nameBytes[:], name)
	arcBuf.Write(nameBytes[:])

	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], uint32(len(crunched)))
	arcBuf.Write(b4[:])

	arcBuf.Write([]byte{0x21, 0x14, 0x48, 0x82})

	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[:], crc)
	arcBuf.Write(b2[:])

	binary.LittleEndian.PutUint32(b4[:], uint32(len(content)))
	arcBuf.Write(b4[:])

	arcBuf.Write(crunched)
	arcBuf.Write([]byte{arcMark, 0})

	dst := "/home/ben/github/archive/testdata/ARC_CRU7.ARC"
	if err := os.WriteFile(dst, arcBuf.Bytes(), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %s with Method 7 (orig=%d, comp=%d)\n", dst, len(content), len(crunched))
}
