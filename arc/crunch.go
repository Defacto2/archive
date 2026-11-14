package arc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	initBits     = 9
	crunchedBits = 12
	squashedBits = 13
	reset        = 256
	first        = 257
)

// Uncrunch decompresses data compressed with the following SEA ARC methods:
//   - Method 8: Dynamic Lempel-Zev packing with adaptive reset
//   - Method 9: Squashing
//
// There is one bool argument, squash.
//
//   - To use method 8: squash=false
//   - To use method 9: squash=true
func Uncrunch(r io.Reader, squash bool) ([]byte, error) { //nolint:funlen
	var c code
	err := c.init(r, squash)
	if err != nil {
		return nil, err
	}

	prefix := make([]int, c.tableSize)
	suffix := make([]byte, c.tableSize)
	for i := range 256 {
		suffix[i] = byte(i)
	}

	const padding = 3
	buf := make([]byte, squashedBits+padding)

	prevCode, err := c.getcode(buf, r)
	if err != nil {
		return nil, err
	}
	if prevCode < 0 || prevCode >= 256 {
		const format = "invalid initial code %d: %w"
		return nil, fmt.Errorf(format, prevCode, ErrCorruptLZW)
	}

	var out bytes.Buffer
	b := byte(prevCode)
	out.WriteByte(b)

	stack := make([]byte, c.tableSize)

	for {
		next, err := c.nextCode(buf, r)
		if err != nil {
			return nil, err
		}
		if next == -1 {
			break
		}

		currCode := next
		var stackPtr int
		b, stackPtr, err = expandCode(next, prevCode, b, c.dictIndex, prefix, suffix, stack)
		if err != nil {
			return nil, err
		}

		for stackPtr > 0 {
			stackPtr--
			out.WriteByte(stack[stackPtr])
		}

		if c.dictIndex < c.tableSize {
			prefix[c.dictIndex] = prevCode
			suffix[c.dictIndex] = b
			c.dictIndex++
		}
		prevCode = currCode
	}

	if squash {
		return out.Bytes(), nil
	}
	return UnpackRLE(out.Bytes()), nil
}

// expandCode returns the first character of the decoded string and the number of decoded bytes.
func expandCode(next, prev int, b byte, freeEnt int, prefix []int, suffix, stack []byte) (byte, int, error) {
	stackPtr := 0
	if next >= freeEnt {
		if next > freeEnt {
			const format = "invalid code %d value, which is > %d free dictionary value: %w"
			return 0, 0, fmt.Errorf(format, next, freeEnt, ErrCorruptLZW)
		}
		stack[stackPtr] = b
		stackPtr++
		next = prev
	}

	for next >= 256 {
		stack[stackPtr] = suffix[next]
		stackPtr++
		next = prefix[next]
	}

	firstChar := suffix[next]
	stack[stackPtr] = firstChar
	stackPtr++

	return firstChar, stackPtr, nil
}

type code struct {
	reset     bool
	bitLen    int // bitLen is the current bit length
	bitLimit  int // bitLimit is the maximum permitted bit-length
	dictIndex int // dictIndex is the index of the next free dictionary entry
	offset    int // offset is the current bit position
	offsetMax int // bufSize is the maximum bit offset
	maximum   int // maximum code value representable at the current bitLen
	tableSize int // tableSize is the total capacity of the dictionary table
}

func (c *code) init(r io.Reader, squash bool) error {
	c.bitLimit = squashedBits
	if !squash {
		c.bitLimit = crunchedBits
		var hdrBits [1]byte
		_, err := io.ReadFull(r, hdrBits[:])
		if err != nil {
			return fmt.Errorf("read crc bits header: %w", err)
		}
		if int(hdrBits[0]) != crunchedBits {
			return fmt.Errorf("%w: unsupported bit length %d", ErrCorruptLZW, hdrBits[0])
		}
	}

	c.tableSize = 1 << c.bitLimit
	c.dictIndex = first
	c.bitLen = initBits
	c.maximum = (1 << initBits) - 1
	return nil
}

// nextCode returns the next unpacked LZW code word and handles any reset markers.
func (c *code) nextCode(buf []byte, r io.Reader) (int, error) {
	val, err := c.getcode(buf, r)
	if err != nil || val != reset {
		return val, err
	}
	c.reset = true
	c.dictIndex = first - 1
	return c.getcode(buf, r)
}

// stepSize adjusts the current bit-length.
func (c *code) stepSize() {
	if c.dictIndex > c.maximum {
		c.bitLen++
		if c.bitLen == c.bitLimit {
			c.maximum = c.tableSize
		} else {
			c.maximum = (1 << c.bitLen) - 1
		}
	}
	if c.reset {
		c.bitLen = initBits
		c.maximum = (1 << c.bitLen) - 1
		c.reset = false
	}
}

// refill reads the next block of bytes into the buffer.
func (c *code) refill(buf []byte, r io.Reader) (bool, error) {
	// clear any trailing padding bytes
	buf[c.bitLen] = 0
	buf[c.bitLen+1] = 0
	buf[c.bitLen+2] = 0

	n, err := io.ReadFull(r, buf[:c.bitLen])
	if n <= 0 {
		return false, nil
	}
	c.offset = 0
	const bitsPerByte = 8
	c.offsetMax = (n * bitsPerByte) - (c.bitLen - 1)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, fmt.Errorf("code refill: %w", err)
	}
	return true, nil
}

// getcode extracts the next variable-length bit code from the buffer.
func (c *code) getcode(buf []byte, r io.Reader) (int, error) {
	if c.reset || c.offset >= c.offsetMax || c.dictIndex > c.maximum {
		c.stepSize()
		ok, err := c.refill(buf, r)
		if err != nil {
			return -1, err
		}
		if !ok {
			return -1, nil
		}
	}

	rOff := c.offset
	bits := c.bitLen

	const rShift = 3
	bp := rOff >> rShift
	rOff &= 7

	code := int(buf[bp] >> rOff)
	bp++
	const bb = 8 // bb is number of bits per byte
	bits -= (bb - rOff)
	rOff = bb - rOff

	if bits >= bb {
		code |= int(buf[bp]) << rOff
		bp++
		rOff += bb
		bits -= bb
	}

	rmask := [9]byte{0x00, 0x01, 0x03, 0x07, 0x0f, 0x1f, 0x3f, 0x7f, 0xff}
	code |= int(buf[bp]&rmask[bits]) << rOff
	c.offset += c.bitLen

	return code & ((1 << c.bitLimit) - 1), nil
}
