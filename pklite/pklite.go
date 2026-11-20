// This package is derived from the PKLITE module of Deark,
// https://github.com/jsummers/deark/blob/master/modules/pklite.c
//
// Copyright (C) 2021 Jason Summers
//
// Deark is distributed under the MIT license, whose terms follow.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package pklite decompresses MS-DOS executables that were packed with PKWARE's PKLITE.
//
// PKLITE was released in 1990 by PKWARE as an executable packer. Unlike the
// legacy ZIP methods in the sibling packages (unshrink, explode, expand), PKLITE
// is not a ZIP compression method and has no method ID, so it cannot be
// registered with [archive/zip]. Instead it is a stand-alone decompressor
// that takes a packed file and returns the original executable.
//
// PKLITE compressed .COM files (v1.00 to v2.01) and .EXE files of v1.05 to v1.12
// are supported. The EXE support is limited to the large and small compression,
// scrambled decompressors and v1.20 files are not yet supported.
//
// The compression is an LZ77 variant, with 16-bit little-endian bit words that
// are consumed from the least significant bit, and static Huffman codes for the
// match lengths and offsets. The format was documented by Jason Summers
// in the MIT licensed [Deark] project, which this package is based on.
//
// [Deark]: https://github.com/jsummers/deark
//
//nolint:gochecknoglobals
package pklite

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

var (
	ErrNotPKLITE = errors.New("pklite: not a supported compressed file")
	ErrCorrupt   = errors.New("pklite: corrupt compressed data")
	ErrTruncated = errors.New("pklite: unexpected end of compressed data")
)

const (
	// MaxSize is the maximum number of bytes that Decompress will return.
	MaxSize = 1 << 20

	comBase    = 0x100 // MS-DOS loads a COM file at offset 0x100 of its segment.
	scanLimit  = 64    // the copier stub is found within the first bytes.
	longLenSym = 7     // the symbol for a long match length, for small COM files.
	longBias   = 10
)

// Huffman codes for the small (COM) compression. The high 4 bits
// are the code length and the low 12 bits are the code.
var largeLengthCodes = [...]uint16{
	0x2003, 0x3000, 0x4002, 0x4003, 0x4004, 0x500a, 0x500b, 0x500c,
	0x601a, 0x601b, 0x703a, 0x703b, 0x703c, 0x807a, 0x807b, 0x807c,
	0x90fa, 0x90fb, 0x90fc, 0x90fd, 0x90fe, 0x90ff, 0x601c, 0x2002,
}

var smallLengthCodes = [...]uint16{
	0x2000, 0x3004, 0x3005, 0x400c, 0x400d, 0x400e, 0x400f, 0x3003,
	0x3002,
}

var offsetCodes = [...]uint16{
	0x1001, 0x4000, 0x4001, 0x5004, 0x5005, 0x5006, 0x5007, 0x6010,
	0x6011, 0x6012, 0x6013, 0x6014, 0x6015, 0x6016, 0x702e, 0x702f,
	0x7030, 0x7031, 0x7032, 0x7033, 0x7034, 0x7035, 0x7036, 0x7037,
	0x7038, 0x7039, 0x703a, 0x703b, 0x703c, 0x703d, 0x703e, 0x703f,
}

// IsCOM reports whether data looks like a PKLITE compressed COM file.
func IsCOM(data []byte) bool {
	_, _, err := findDataPos(data)
	return err == nil
}

// Decompress returns the original executable from a PKLITE compressed COM or EXE file.
func Decompress(data []byte) ([]byte, error) {
	if IsEXE(data) {
		return decompressEXE(data)
	}
	pos, opt, err := findDataPos(data)
	if err != nil {
		return nil, err
	}
	out, _, err := decode(data, pos, opt)
	return out, err
}

// Read reads all of r and returns the decompressed COM file.
func Read(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("pklite read: %w", err)
	}
	return Decompress(data)
}

// options are the decompression variants.
type options struct {
	large, extra bool
	prehistory   bool
}

// findDataPos returns the file offset of the compressed data in a COM file.
//
// PKLITE COM files begin with a small intro, then a copier that moves
// the decompressor to the top of memory, and then the decompressor that holds the
// address of the compressed data. Some tools overwrite the intro, so the copier
// is searched for rather than expected at a fixed offset.
func findDataPos(data []byte) (int, options, error) {
	var o options
	if len(data) < 128 {
		return 0, o, ErrNotPKLITE
	}
	if data[0] == 0xba && data[3] == 0xa1 && data[6] == 0x2d && data[7] == 0x20 {
		return findBetaDataPos(data)
	}
	head := data[:min(len(data), scanLimit)]
	if !bytes.Contains(data[:min(len(data), 0x80)], []byte("PKLITE")) &&
		!bytes.Contains(data[:min(len(data), 0x80)], []byte("PKlite")) {
		return 0, o, ErrNotPKLITE
	}
	// copier: and ax,0xfff0; mov di,ax; mov cx,N; [nop]; mov si,decompressor
	pre := []byte{0x25, 0xf0, 0xff, 0x8b, 0xf8, 0xb9}
	i := bytes.Index(head, pre)
	if i < 0 {
		return 0, o, ErrNotPKLITE
	}
	p := i + len(pre) + 2 // after the cx count
	if p < len(head) && head[p] == 0x90 {
		p++
	}
	if p+2 >= len(data) || data[p] != 0xbe {
		return 0, o, ErrNotPKLITE
	}
	decomp := int(data[p+1]) | int(data[p+2])<<8 - comBase
	// decompressor: std; mov di,ax; dec di; dec di; mov si,KEY
	sig := []byte{0xfd, 0x8b, 0xf8, 0x4f, 0x4f, 0xbe}
	if decomp < 0 || decomp+len(sig)+2 > len(data) || !bytes.Equal(data[decomp:decomp+len(sig)], sig) {
		return 0, o, ErrNotPKLITE
	}
	k := decomp + len(sig)
	pos := (int(data[k]) | int(data[k+1])<<8) + 2 - comBase
	if pos < 1 || pos >= len(data) {
		return 0, o, ErrNotPKLITE
	}
	return pos, o, nil
}

// findBetaDataPos is for the PKLITE v1.00 beta COM files that have a different stub:
// an intro that jumps to the copier, which is followed by the decompressor.
func findBetaDataPos(data []byte) (int, options, error) {
	o := options{prehistory: true}
	// the intro: mov dx,N; mov ax,[2]; sub ax,N; mov bx,cs; add bx,N; cmp ax,bx; ja
	if find(data, 0, 18, "\xba??\xa1\x02\x00\x2d??\x8c\xcb??????\x77") != 0 || !bytes.Contains(data[:0x50], []byte("PK Copyr")) {
		return 0, o, ErrNotPKLITE
	}
	pos := 18 + 1 + int(data[18]) // follow the one byte jump
	// copier: cli; mov sp,0x200; mov ss,ax; sti
	if pos+24+10 > len(data) || find(data, pos, pos+7, "\xfa\xbc\x00\x02\x8e\xd0\xfb") != pos {
		return 0, o, ErrNotPKLITE
	}
	decomp := pos + 24
	// decompressor: std; mov si,KEY; add si,dx; mov di,dx; dec di; dec di
	if find(data, decomp, decomp+10, "\xfd\xbe??\x03\xf2\x8b\xfa\x4f\x4f") != decomp {
		return 0, o, ErrNotPKLITE
	}
	start := int(data[decomp+2]) | int(data[decomp+3])<<8 + 2 - comBase
	if start < 1 || start >= len(data) {
		return 0, o, ErrNotPKLITE
	}
	return start, o, nil
}

type huffman map[uint16]uint16 // (length<<12|code) to symbol

func newHuffman(codes []uint16) huffman {
	h := make(huffman, len(codes))
	for sym, lc := range codes {
		h[lc] = uint16(sym)
	}
	return h
}

type bitReader struct {
	data []byte
	pos  int
	bits uint16
	n    uint
	eof  bool
}

func (b *bitReader) fill() bool {
	if b.pos+2 > len(b.data) {
		return false
	}
	b.bits = uint16(b.data[b.pos]) | uint16(b.data[b.pos+1])<<8
	b.pos += 2
	b.n = 16
	return true
}

// bit consumes the next bit, refilling eagerly like the original decompressor.
// Running out of data is only an error when a bit is actually needed.
func (b *bitReader) bit() int {
	if b.n == 0 && !b.fill() {
		b.eof = true
		return 0
	}
	v := int(b.bits & 1)
	b.bits >>= 1
	b.n--
	if b.n == 0 {
		b.fill()
	}
	return v
}

func (b *bitReader) byte() (byte, bool) {
	if b.pos >= len(b.data) {
		return 0, false
	}
	v := b.data[b.pos]
	b.pos++
	return v, true
}

func (b *bitReader) symbol(h huffman) (int, error) {
	var code, length uint16
	for length < 12 {
		code = code<<1 | uint16(b.bit()) //nolint:gosec
		length++
		if b.eof {
			return 0, ErrTruncated
		}
		if sym, ok := h[length<<12|code]; ok {
			return int(sym), nil
		}
	}
	return 0, ErrCorrupt
}

// decode decompresses the PKLITE data beginning at start.
// It returns the decompressed bytes and the offset following the compressed data.
// The options select the codes used by the large (EXE) compression,
// the obfuscation of the literal bytes, and the v1.00 beta implied zero byte before the data.
func decode(data []byte, start int, o options) ([]byte, int, error) {
	large, extra := o.large, o.extra
	br := &bitReader{data: data, pos: start}
	if !br.fill() {
		return nil, 0, ErrTruncated
	}
	codes, longSym, bias := smallLengthCodes[:], 7, 10
	if large {
		codes, longSym, bias = largeLengthCodes[:], 22, 25
	}
	ml2Sym := longSym + 1 // a two byte match, with the offset high bits of zero
	lengths := newHuffman(codes)
	offsets := newHuffman(offsetCodes[:])
	out := make([]byte, 0, 4096)

	for {
		flag := br.bit()
		if br.eof {
			return nil, 0, ErrTruncated
		}
		if flag == 0 {
			c, ok := br.byte()
			if !ok {
				return nil, 0, ErrTruncated
			}
			if extra {
				c ^= byte(br.n) //nolint:gosec
			}
			out = append(out, c)
			if len(out) > MaxSize {
				return nil, 0, ErrCorrupt
			}
			continue
		}
		sym, err := br.symbol(lengths)
		if err != nil {
			return nil, 0, err
		}
		var length, hi int
		haveHi := false
		switch {
		case sym < longSym:
			length = sym + 3
		case sym == longSym:
			b, ok := br.byte()
			if !ok {
				return nil, 0, ErrTruncated
			}
			switch {
			case b == 0xff: // the stop code
				return out, br.pos, nil
			case b == 0xfe && large: // a segment separator
				continue
			case b >= 0xfd:
				return nil, 0, fmt.Errorf("%w: unsupported code 0x%02x", ErrCorrupt, b)
			}
			length = int(b) + bias
		case sym == ml2Sym:
			length = 2
			haveHi = true
		default:
			return nil, 0, ErrCorrupt
		}
		if !haveHi {
			if hi, err = br.symbol(offsets); err != nil {
				return nil, 0, err
			}
		}
		lo, ok := br.byte()
		if !ok {
			return nil, 0, ErrTruncated
		}
		dist := hi<<8 | int(lo)
		if len(out)+length > MaxSize {
			return nil, 0, ErrCorrupt
		}
		if o.prehistory && dist == len(out)+1 {
			// the v1.00 beta may copy from an implied zero byte before the data.
			out = append(out, 0)
			length--
			dist = len(out)
		}
		if dist == 0 || dist > len(out) {
			return nil, 0, fmt.Errorf("%w: distance %d with %d bytes written", ErrCorrupt, dist, len(out))
		}
		for range length {
			out = append(out, out[len(out)-dist])
		}
	}
}
