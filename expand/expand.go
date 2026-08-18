// Package expand is for use with the [archive/zip] package.
//
// Expand was a decompression method authored in 1989 by Phil Katz
// and used in his MS-DOS compression tool, PKZip v0.90.
//
// It is referred to as ZIP Reduce/Expand, methods 2, 3, 4, 5,
// and is based on the LZ77 algorithm.
//
// This package was first prompted using Gemini in August 2026 and
// manually cleaned to be idiomatic and readable.
//
// In 2020, Jason Summers authored a collection of public domain libraries named [oldunzip].
// In-depth [information] on Reduce was authored by Hans Wennborg in 2021,
// who programmed the public domain [reduce.c].
//
// [oldunzip]: https://github.com/jsummers/oldunzip
// [information]: https://www.hanshq.net/zip2.html#reduce
// [reduce.c]: https://www.hanshq.net/files/hwzip/reduce.c
package expand

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
)

var (
	// ErrCorruptData is returned when the compressed data stream is malformed.
	ErrCorruptData = errors.New("expand: corrupt compressed data")
	// ErrMethod is returned when an invalid compression method is specified.
	ErrMethod = errors.New("expand: invalid method for expand dcomp")
	// ErrOverflow is returned when an integer cannot be safely wrapped to a new type.
	ErrOverflow = errors.New("expand: integer overflow")
)

const (
	Expand2 uint16 = iota + 2 // Method 2: Reduced/Expand with compression factor 1
	Expand3                   // Method 3: Reduced/Expand with compression factor 2
	Expand4                   // Method 4: Reduced/Expand with compression factor 3
	Expand5                   // Method 5: Reduced/Expand with compression factor 4
)

const (
	dleByte    = 144
	windowSize = 4096
)

var registerOnce sync.Once //nolint:gochecknoglobals

// Register the [expand] decompressor methods globally for [archive/zip].
func Register() {
	registerOnce.Do(func() {
		zip.RegisterDecompressor(Expand2, dcomp2)
		zip.RegisterDecompressor(Expand3, dcomp3)
		zip.RegisterDecompressor(Expand4, dcomp4)
		zip.RegisterDecompressor(Expand5, dcomp5)
	})
}

type followerSet struct {
	size      uint8
	idxBits   uint8
	followers [32]byte
}

type expand struct {
	r             *bufio.Reader
	factor        int
	bits          uint32
	bitCount      uint
	followers     [256]followerSet
	followersRead bool
	prevByte      byte
	window        [windowSize]byte
	winPos        int
	totalWritten  int64
	buf           []byte
	bufPos        int
	readErr       error
}

// dcomp returns an [io.ReadCloser] that decodes methods 2, 3, 4, or 5 (Expand) streams.
func dcomp(r io.Reader, method uint16) io.ReadCloser {
	const size = 512
	factor := int(method - 1)
	return &expand{
		r:             bufio.NewReader(r),
		factor:        factor,
		bits:          0,
		bitCount:      0,
		followers:     [256]followerSet{},
		followersRead: false,
		prevByte:      0,
		window:        [windowSize]byte{},
		winPos:        0,
		totalWritten:  0,
		buf:           make([]byte, 0, size),
		bufPos:        0,
		readErr:       nil,
	}
}

func dcomp2(r io.Reader) io.ReadCloser {
	return dcomp(r, Expand2)
}

func dcomp3(r io.Reader) io.ReadCloser {
	return dcomp(r, Expand3)
}

func dcomp4(r io.Reader) io.ReadCloser {
	return dcomp(r, Expand4)
}

func dcomp5(r io.Reader) io.ReadCloser {
	return dcomp(r, Expand5)
}

func (e *expand) Close() error {
	return nil
}

func (e *expand) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if !e.followersRead {
		if err := e.loadFollowers(); err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}
	}

	for e.bufPos >= len(e.buf) {
		if e.readErr != nil {
			return 0, e.readErr
		}
		e.buf = e.buf[:0]
		e.bufPos = 0

		err := e.next()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				e.readErr = io.EOF
				if len(e.buf) == 0 {
					return 0, io.EOF
				}
				break
			}
			e.readErr = err
			if len(e.buf) == 0 {
				return 0, err
			}
			break
		}
	}

	n := copy(p, e.buf[e.bufPos:])
	e.bufPos += n
	return n, nil
}

func (e *expand) loadFollowers() error {
	const format = "expand load followers %s: %w"
	const bits = 6
	const maxBytes = 32
	for i := 255; i >= 0; i-- {
		n, err := e.code(bits)
		if err != nil {
			return err
		}
		if n > maxBytes {
			return ErrCorruptData
		}
		e.followers[i].size = uint8(n)
		e.followers[i].idxBits = followerBits(int(n))
		const bits = 8
		for j := range n {
			b, err := e.code(bits)
			if err != nil {
				return err
			}
			if b > math.MaxUint8 {
				return fmt.Errorf(format,
					fmt.Sprintf("%d exceeds uint8 maximum %d", bits, math.MaxUint8), ErrOverflow)
			}
			e.followers[i].followers[j] = byte(b)
		}
	}
	e.followersRead = true
	return nil
}

func followerBits(n int) uint8 {
	const (
		b16      = 16
		b8       = 8
		b4       = 4
		b2       = 2
		b0       = 0
		indices5 = 5
		indices4 = 4
		indices3 = 3
		indices2 = 2
		indices1 = 1
	)
	switch {
	case n > b16:
		return indices5
	case n > b8:
		return indices4
	case n > b4:
		return indices3
	case n > b2:
		return indices2
	case n > b0:
		return indices1
	default:
		return 0
	}
}

func (e *expand) code(bits uint) (uint32, error) {
	const format = "expand code %s: %w"
	for e.bitCount < bits {
		b, err := e.r.ReadByte()
		if err != nil {
			return 0, fmt.Errorf(format, "byte", err)
		}
		e.bits |= uint32(b) << e.bitCount
		e.bitCount += 8
	}
	val := e.bits & ((1 << bits) - 1)
	e.bits >>= bits
	e.bitCount -= bits
	return val, nil
}

func (e *expand) traverse() (byte, error) {
	const format = "expand traverse %s: %w"
	fset := &e.followers[e.prevByte]
	var b byte

	if fset.size == 0 {
		const bits = 8
		n, err := e.code(bits)
		if err != nil {
			return 0, err
		}
		if n > math.MaxUint8 {
			return 0, fmt.Errorf(format,
				fmt.Sprintf("%d exceeds uint8 maximum %d", n, math.MaxUint8), ErrOverflow)
		}
		b = byte(n)
		e.prevByte = b
		return b, nil
	}
	const bits = 1
	tag, err := e.code(bits)
	if err != nil {
		return 0, err
	}
	if tag == 1 {
		const bits = 8
		n, err := e.code(bits)
		if err != nil {
			return 0, err
		}
		if n > math.MaxUint8 {
			return 0, fmt.Errorf(format,
				fmt.Sprintf("%d exceeds uint8 maximum %d", n, math.MaxUint8), ErrOverflow)
		}
		b = byte(n)
		e.prevByte = b
		return b, nil
	}

	idx, err := e.code(uint(fset.idxBits))
	if err != nil {
		return 0, err
	}
	if idx >= uint32(fset.size) {
		return 0, ErrCorruptData
	}
	b = fset.followers[idx]
	e.prevByte = b
	return b, nil
}

func (e *expand) next() error {
	const format = "expand next %s: %w"
	b, err := e.traverse()
	if err != nil {
		return err
	}

	if b != dleByte {
		e.emitByte(b)
		return nil
	}

	v, err := e.traverse()
	if err != nil {
		return err
	}

	if v == 0 {
		e.emitByte(dleByte)
		return nil
	}

	const bits = 8
	vLenBits := uint(bits - e.factor)
	vLenMask := uint32((1 << vLenBits) - 1)

	matchLen := int(uint32(v) & vLenMask)
	if matchLen > math.MaxUint32 {
		return fmt.Errorf(format,
			fmt.Sprintf("%d exceeds uint32 maximum %d", matchLen, math.MaxUint32), ErrOverflow)
	}

	if uint32(matchLen) == vLenMask {
		extraLen, err := e.traverse()
		if err != nil {
			return err
		}
		matchLen += int(extraLen)
	}
	matchLen += 3

	w, err := e.traverse()
	if err != nil {
		return err
	}
	matchDist := int((uint32(v)>>vLenBits)*256 + uint32(w) + 1)

	for range matchLen {
		var out byte
		if int64(matchDist) > e.totalWritten {
			out = 0
		} else {
			copyPos := (e.winPos - matchDist + windowSize) % windowSize
			out = e.window[copyPos]
		}
		e.emitByte(out)
	}

	return nil
}

func (e *expand) emitByte(b byte) {
	e.window[e.winPos] = b
	e.winPos = (e.winPos + 1) % windowSize
	e.totalWritten++
	e.buf = append(e.buf, b)
}
