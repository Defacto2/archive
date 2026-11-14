package arc

import (
	"bytes"
	"fmt"
	"io"
)

const (
	oldTabSize    = 4096
	noPredecessor = 0xFFFF
	mask16        = 0xFFFF
	tableMask     = 0x0FFF
)

type hashFunc func(predecessor uint16, follower byte) uint16

// repeatFn calculates the 12-bit middle square hash for Method 5 and 6.
func repeatFn(pred uint16, foll byte) uint16 {
	const (
		floor  = 0x0800
		rshift = 6
	)
	local := (uint32(pred+uint16(foll)) | floor) & mask16
	local *= local
	return uint16((local >> rshift) & tableMask)
}

// williamFn calculates the faster hash for Method 7.
func williamFn(pred uint16, foll byte) uint16 {
	const scrambler = 15073
	return uint16(((uint32(pred+uint16(foll)) & mask16) * scrambler) & tableMask)
}

type entry struct {
	used        bool
	follower    byte
	next        uint16
	predecessor uint16
}

type entrys []entry

func newEntrys(fn hashFunc) entrys {
	e := make(entrys, oldTabSize)
	for i := range 256 {
		e.update(fn, noPredecessor, byte(i))
	}
	return e
}

// eolist is the end of list.
func (e entrys) eolist(index uint16) uint16 {
	for e[index].next != 0 {
		index = e[index].next
	}
	return index
}

// hashLookup finds the dictionary table index for a predecessor and follower.
func (e entrys) hashLookup(fn hashFunc, predecessor uint16, follower byte) uint16 {
	local := fn(predecessor, follower)
	if !e[local].used {
		return local
	}

	last := e.eolist(local)
	const probe = 101
	next := (last + probe) & tableMask
	for e[next].used {
		next = (next + 1) & tableMask
	}
	e[last].next = next
	return next
}

// update adds a new string entry with its predecessor and follower.
func (e entrys) update(fn hashFunc, predecessor uint16, follower byte) {
	idx := e.hashLookup(fn, predecessor, follower)
	e[idx] = entry{
		used:        true,
		follower:    follower,
		next:        0,
		predecessor: predecessor,
	}
}

func (e entrys) expand(next, prevCode int, b byte, stack []byte) (byte, []byte) {
	if !e[next].used {
		next = prevCode
		stack = append(stack, b)
	}

	for e[next].predecessor != noPredecessor {
		stack = append(stack, e[next].follower)
		next = int(e[next].predecessor)
	}
	b = e[next].follower
	stack = append(stack, b)
	return b, stack
}

// codeReader unpacks 12-bit code pairs from a byte stream.
type codeReader struct {
	data   []byte
	inPos  int
	inFlag bool
}

func (cr *codeReader) next() (int, bool) {
	const (
		shift    = 4
		rshift   = 4
		lowMask  = 0x0f
		lowShift = 8
	)
	if !cr.inFlag {
		if cr.inPos+1 >= len(cr.data) {
			return -1, false
		}
		code := (int(cr.data[cr.inPos]) << shift) | (int(cr.data[cr.inPos+1]) >> rshift)
		cr.inPos++
		cr.inFlag = true
		return code, true
	}
	if cr.inPos+1 >= len(cr.data) {
		return -1, false
	}
	code := (int(cr.data[cr.inPos]&lowMask) << lowShift) | int(cr.data[cr.inPos+1])
	cr.inPos += 2
	cr.inFlag = false
	return code, true
}

// UncrunchLZW decompresses data compressed with the following SEA ARC methods:
//   - Method 5: Lempel-Zev packing of repeated strings (old style)
//   - Method 6: Lempel-Zev packing of repeated strings (new style)
//   - Method 7: Lempel-Zev Williams packing with improved hash function
//
// There are two bool arguments, william and rle.
//
//   - To use method 5, william=false, rle=false
//   - To use method 6, william=false, rle=true
//   - To use method 7, william=true, rle=true
func UncrunchLZW(r io.Reader, william, rle bool) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read crc bits header: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	hashFn := repeatFn
	if william {
		hashFn = williamFn
	}

	e := newEntrys(hashFn)
	const charCnt = 256
	codeCount := oldTabSize - charCnt
	cr := codeReader{data: data, inPos: 0, inFlag: false}

	prevCode, ok := cr.next()
	if !ok || prevCode < 0 || prevCode >= oldTabSize {
		const format = "invalid initial code %d: %w"
		return nil, fmt.Errorf(format, prevCode, ErrCorruptLZW)
	}

	b := e[prevCode].follower
	var buf bytes.Buffer
	buf.WriteByte(b)

	stack := make([]byte, 0, oldTabSize)

	for {
		next, ok := cr.next()
		if !ok || next < 0 || next >= oldTabSize {
			break
		}

		b, stack = e.expand(next, prevCode, b, stack[:0])

		for len(stack) > 0 {
			n := len(stack) - 1
			buf.WriteByte(stack[n])
			stack = stack[:n]
		}

		if codeCount > 0 {
			e.update(hashFn, uint16(prevCode), b)
			codeCount--
		}
		prevCode = next
	}

	if rle {
		return UnpackRLE(buf.Bytes()), nil
	}
	return buf.Bytes(), nil
}
