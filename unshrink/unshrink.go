// Package unshrink is for use with the [archive/zip] package.
//
// Unshrink was a decompression method authored in 1989 by Phil Katz
// and used in his MS-DOS compression tool, PKZip v0.80 and v0.90.
//
// It is retroactively referred to as ZIP Method 1 and is based on the LZW algorithm.
//
// This package was first prompted using Gemini in August 2026 and
// manually cleaned to be idiomatic and readable.
//
// In 2020, Jason Summers authored a collection of public domain libraries named [oldunzip].
// [In-depth information] on Shrink was authored by Hans Wennborg in 2021.
//
// [oldunzip]: https://github.com/jsummers/oldunzip
// [In-depth information]: https://www.hanshq.net/zip2.html#shrink
package unshrink

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
)

var ErrOverflow = errors.New("integer overflow")

const Unshrink uint16 = 1

// Register the [Unshrink] method globally for [archive/zip].
func Register() {
	zip.RegisterDecompressor(Unshrink, dcomp)
}

const (
	controlCode  = 256
	initFreeCode = 257
	maxCode      = 8192
	minCodeSize  = 9
)

type unshrink struct {
	r         io.ByteReader
	stackTop  int
	bitCount  uint
	codeSize  uint
	bits      uint32
	freeCode  uint16
	lastCode  uint16
	firstChar byte
	prefix    [8192]uint16
	suffix    [8192]byte
	stack     [8192]byte
	used      [8192]bool
}

// dcomp returns an [io.ReadCloser] that decodes method 1 (Unshrink) streams.
func dcomp(r io.Reader) io.ReadCloser {
	br, ok := r.(io.ByteReader)
	if !ok {
		br = bufio.NewReader(r)
	}

	u := &unshrink{
		r:         br,
		stackTop:  0,
		bitCount:  0,
		codeSize:  minCodeSize,
		bits:      0,
		freeCode:  initFreeCode,
		lastCode:  math.MaxUint16,
		firstChar: 0,
		prefix:    [8192]uint16{},
		suffix:    [8192]byte{},
		stack:     [8192]byte{},
		used:      [8192]bool{},
	}

	// empty prefix table
	for i := range maxCode {
		u.prefix[i] = math.MaxUint16
	}
	// root table entries
	for i := range controlCode {
		u.suffix[i] = byte(i)
	}

	return u
}

func (u *unshrink) Close() error {
	return nil
}

func (u *unshrink) Read(p []byte) (n int, err error) {
	for n < len(p) {
		if flush := u.stackTop > 0; flush {
			u.stackTop--
			p[n] = u.stack[u.stackTop]
			n++
			continue
		}

		code, err := u.code()
		if err != nil {
			return n, err
		}

		if code == controlCode {
			if n, err = u.code256(n); err != nil {
				return n, err
			}
			continue
		}

		currentCode := code
		code = u.kwProblem(code)
		u.traverse(code)
		u.update()
		u.lastCode = currentCode
	}
	return n, nil
}

// kwProblem handles an edge case [documented] as the LZW "KwKwK problem".
//
// documented: https://www.hanshq.net/zip2.html#lzwalg
func (u *unshrink) kwProblem(code uint16) uint16 {
	if code >= initFreeCode && u.prefix[code] == math.MaxUint16 {
		u.stack[u.stackTop] = u.firstChar
		u.stackTop++
		code = u.lastCode
	}
	return code
}

// traverse dictionary tree up to root.
func (u *unshrink) traverse(code uint16) {
	for code >= controlCode && code != math.MaxUint16 {
		u.stack[u.stackTop] = u.suffix[code]
		u.stackTop++
		code = u.prefix[code]
	}
	if code != math.MaxUint16 {
		u.firstChar = u.suffix[code]
		u.stack[u.stackTop] = u.firstChar
		u.stackTop++
	}
}

// update LZW dictionary entry.
func (u *unshrink) update() {
	if u.lastCode != math.MaxUint16 && u.freeCode < maxCode {
		u.prefix[u.freeCode] = u.lastCode
		u.suffix[u.freeCode] = u.firstChar

		// scan forward to find the next available free code
		for u.freeCode < maxCode && u.prefix[u.freeCode] != math.MaxUint16 {
			u.freeCode++
		}
	}
}

// code256 is the control code for bit expansion or partial clearing of the dictionary.
func (u *unshrink) code256(n int) (int, error) {
	code, err := u.code()
	if err != nil {
		return n, err
	}
	const (
		bitExpansion = 1
		partialClear = 2
	)
	switch code {
	case bitExpansion:
		u.codeSize++
	case partialClear:
		u.partialClear()
	}
	return n, nil
}

// partialClear removes all the strings in the dictionary that are not prefixes of other strings.
func (u *unshrink) partialClear() {
	clear(u.used[:])

	// scan for codes that have been used as a prefix
	for i := uint16(initFreeCode); i < maxCode; i++ {
		p := u.prefix[i]
		if used := p != math.MaxUint16; used {
			u.used[p] = true
		}
	}

	for i := uint16(initFreeCode); i < maxCode; i++ {
		if clearPrefix := !u.used[i]; clearPrefix {
			u.prefix[i] = math.MaxUint16
		}
	}

	// reset freeCode to the first available slot
	u.freeCode = initFreeCode
	for u.freeCode < maxCode && u.prefix[u.freeCode] != math.MaxUint16 {
		u.freeCode++
	}
}

func (u *unshrink) code() (uint16, error) {
	const format = "unshrink code %s: %w"
	for u.bitCount < u.codeSize {
		b, err := u.r.ReadByte()
		if err != nil {
			return 0, fmt.Errorf(format, "byte", err)
		}
		u.bits |= uint32(b) << u.bitCount
		u.bitCount += 8
	}
	mask := uint32((1 << u.codeSize) - 1)
	bits := u.bits & mask
	if bits > math.MaxUint16 {
		return 0, fmt.Errorf(format,
			fmt.Sprintf("%d exceeds uint16 maximum %d", bits, math.MaxUint16), ErrOverflow)
	}
	code := uint16(bits)
	u.bits >>= u.codeSize
	u.bitCount -= u.codeSize
	return code, nil
}
