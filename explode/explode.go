// Package explode is for use with the [archive/zip] package.
//
// Explode was a decompression method authored in 1989 by Phil Katz
// and used in his MS-DOS compression tool, PKZip v0.91.
//
// It is referred to as ZIP Method 6 and is based on the LZ77 algorithm.
//
// This package was first prompted using Gemini in August 2026 and
// manually cleaned to be idiomatic and readable.
//
// In 2020, Jason Summers authored a collection of public domain libraries named [oldunzip].
// [In-depth information] on Implode was authored by Hans Wennborg in 2021.
//
// [oldunzip]: https://github.com/jsummers/oldunzip
// [In-depth information]: https://www.hanshq.net/zip2.html#implode
package explode

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
)

const Explode uint16 = 6 // Method 6: Implode/Explode

const (
	dictionary4K = 4_096
	maxDictSize  = 8192

	symbolsLiteral  = 256
	symbolsLength   = 64
	symbolsDistance = 64

	twoTrees   = 2
	threeTrees = 3

	byteBits   = 8
	distBits4K = 6
	distBits8K = 7

	minMatch2Trees = 2
	minMatch3Trees = 3

	maxCodeLen   = 16
	maxLenSymbol = 63

	lengthNibbleMask = 0x0F
	countShift       = 4

	byteMask = 0xFF
)

var (
	ErrCorruptHeader = errors.New("explode: corrupt Shannon-Fano tree header")
	ErrCorruptStream = errors.New("explode: corrupt compressed stream")
)

//nolint:gochecknoglobals
var registerOnce sync.Once

// Register the [Explode] method globally for [archive/zip].
func Register() {
	registerOnce.Do(func() {
		zip.RegisterDecompressor(Explode, dcomp)
	})
}

type explode struct {
	r          io.ByteReader
	bitBuf     uint32
	bitCount   uint
	dictionary int
	treeCount  int
	position   int

	matchLen  int
	matchDist int

	literals  *sfTree
	lengths   *sfTree
	distances *sfTree

	window   [maxDictSize]byte
	initDone bool
	err      error
}

func dcomp(r io.Reader) io.ReadCloser {
	return newReader(r, 0, 0)
}

// newReader returns an [io.ReadCloser] that decompresses method 6 (Implode/Explode) streams.
// If the provided dictionary size is 0, it defaults to 4096 (4K).
// If the number of trees is 0, it is auto-detected from the stream.
func newReader(r io.Reader, size, trees int) io.ReadCloser {
	br, ok := r.(io.ByteReader)
	if !ok {
		br = bufio.NewReader(r)
	}

	if size <= 0 {
		size = dictionary4K
	}

	return &explode{
		r:          br,
		bitBuf:     0,
		bitCount:   0,
		dictionary: size,
		treeCount:  trees,
		position:   0,
		matchLen:   0,
		matchDist:  0,
		literals:   nil,
		lengths:    nil,
		distances:  nil,
		window:     [maxDictSize]byte{},
		initDone:   false,
		err:        nil,
	}
}

func (e *explode) Close() error {
	return nil
}

func (e *explode) Read(p []byte) (int, error) {
	if err := e.init(); err != nil {
		return 0, err
	}

	var n int
	for n < len(p) {
		if e.matchLen > 0 {
			n = e.pending(p, n)
			continue
		}

		bit, err := e.code(1)
		if err != nil {
			if n > 0 && errors.Is(err, io.EOF) {
				return n, nil
			}
			return n, err
		}

		if bit == 1 {
			b, err := e.literal()
			if err != nil {
				return n, err
			}
			e.window[e.position] = b
			e.position = (e.position + 1) % e.dictionary
			p[n] = b
			n++
		} else {
			if err := e.match(); err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

func (e *explode) pending(p []byte, n int) int {
	copyPos := (e.position - e.matchDist - 1) % e.dictionary
	if copyPos < 0 {
		copyPos += e.dictionary
	}
	b := e.window[copyPos]
	e.window[e.position] = b
	e.position = (e.position + 1) % e.dictionary

	p[n] = b
	e.matchLen--
	return n + 1
}

func (e *explode) literal() (byte, error) {
	if e.treeCount == threeTrees {
		val, err := e.literals.decode(e)
		if err != nil {
			return 0, err
		}
		return byte(val & byteMask), nil
	}

	lit, err := e.code(byteBits)
	if err != nil {
		return 0, err
	}
	return byte(lit & byteMask), nil
}

func (e *explode) match() error {
	distBits := uint(distBits4K)
	if e.dictionary == maxDictSize {
		distBits = distBits8K
	}

	lowDist, err := e.code(distBits)
	if err != nil {
		return err
	}

	highDist, err := e.distances.decode(e)
	if err != nil {
		return err
	}

	distance := int((uint32(highDist) << distBits) | lowDist)

	length, err := e.length()
	if err != nil {
		return err
	}

	e.matchLen = length
	e.matchDist = distance
	return nil
}

func (e *explode) length() (int, error) {
	val, err := e.lengths.decode(e)
	if err != nil {
		return 0, err
	}

	length := int(val)
	if length == maxLenSymbol {
		extra, err := e.code(byteBits)
		if err != nil {
			return 0, err
		}
		length += int(extra)
	}

	minMatch := minMatch2Trees
	if e.treeCount == threeTrees {
		minMatch = minMatch3Trees
	}

	return length + minMatch, nil
}

func (e *explode) init() error {
	if e.initDone {
		return e.err
	}
	e.initDone = true

	count, err := e.treeLength()
	if err != nil {
		const format = "could not decode the tree length: %w"
		e.err = fmt.Errorf(format, err)
		return e.err
	}

	l := len(count)
	if e.treeCount == 0 {
		switch l {
		case symbolsLiteral:
			e.treeCount = threeTrees
		case symbolsLength:
			e.treeCount = twoTrees
		default:
			const format = "unrecognized tree length %d: %w"
			e.err = fmt.Errorf(format, l, ErrCorruptHeader)
			return e.err
		}
	}

	e.err = e.initTrees(count)
	return e.err
}

func (e *explode) initTrees(count []uint8) error {
	lengths := count
	if e.treeCount == threeTrees {
		var err error
		e.literals, err = newTree(lengths, symbolsLiteral)
		if err != nil {
			return err
		}
		lengths, err = e.treeLength()
		if err != nil {
			return err
		}
	}

	var err error
	e.lengths, err = newTree(lengths, symbolsLength)
	if err != nil {
		return err
	}

	distances, err := e.treeLength()
	if err != nil {
		return err
	}
	e.distances, err = newTree(distances, symbolsDistance)
	return err
}

func (e *explode) treeLength() ([]uint8, error) {
	code, err := e.code(byteBits)
	if err != nil {
		return nil, err
	}
	codes := int(code) + 1

	var lengths []uint8
	for range codes {
		b, err := e.code(byteBits)
		if err != nil {
			return nil, err
		}
		count := int((b>>countShift)&lengthNibbleMask) + 1
		length := uint8((b & lengthNibbleMask) + 1)

		for range count {
			lengths = append(lengths, length)
		}
	}

	return lengths, nil
}

func (e *explode) code(n uint) (uint32, error) {
	for e.bitCount < n {
		b, err := e.r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.EOF
			}
			return 0, fmt.Errorf("%w", err)
		}
		e.bitBuf |= uint32(b) << e.bitCount
		e.bitCount += byteBits
	}
	mask := uint32((1 << n) - 1)
	val := e.bitBuf & mask
	e.bitBuf >>= n
	e.bitCount -= n
	return val, nil
}

type node struct {
	left   int16 // Index of left child node (bit 0), or -1 if none
	right  int16 // Index of right child node (bit 1), or -1 if none
	symbol int16 // Decoded symbol (>= 0); -1 if internal node
}

type sfTree struct {
	nodes []node
}

func newTree(lengths []uint8, expectedSymbols int) (*sfTree, error) {
	const format = "expected %d but got %d symbols: %w"
	length := len(lengths)
	if length != expectedSymbols {
		return nil, fmt.Errorf(format, expectedSymbols, length, ErrCorruptHeader)
	}
	return buildTree(lengths)
}

func buildTree(lengths []uint8) (*sfTree, error) {
	tree := &sfTree{
		nodes: []node{{left: -1, right: -1, symbol: -1}},
	}

	var count [maxCodeLen + 1]int
	for _, l := range lengths {
		if l > 0 && l <= maxCodeLen {
			count[l]++
		}
	}

	var code [maxCodeLen + 1]uint32
	var n uint32
	for bits := 1; bits <= maxCodeLen; bits++ {
		n = (n + uint32(count[bits-1]&math.MaxInt32)) << 1
		code[bits] = n
	}

	for symbol, length := range lengths {
		if length > 0 && length <= maxCodeLen {
			tree.insert(uint16(symbol), length, code[length])
			code[length]++
		}
	}

	return tree, nil
}

func (t *sfTree) insert(symbol uint16, length uint8, code uint32) {
	idx := 0
	none := node{-1, -1, -1}
	for i := int(length) - 1; i >= 0; i-- {
		bit := (code >> uint(i)) & 1
		if bit == 0 {
			if t.nodes[idx].left == -1 {
				t.nodes[idx].left = int16(len(t.nodes) & math.MaxInt16)
				t.nodes = append(t.nodes, none)
			}
			idx = int(t.nodes[idx].left)
			continue
		}
		if t.nodes[idx].right == -1 {
			t.nodes[idx].right = int16(len(t.nodes) & math.MaxInt16)
			t.nodes = append(t.nodes, none)
		}
		idx = int(t.nodes[idx].right)
	}
	t.nodes[idx].symbol = int16(symbol & math.MaxInt16)
}

func (t *sfTree) decode(ir *explode) (uint16, error) {
	idx := 0
	for t.nodes[idx].symbol < 0 {
		bit, err := ir.code(1)
		if err != nil {
			return 0, err
		}
		bit ^= 1
		if bit == 0 {
			idx = int(t.nodes[idx].left)
		} else {
			idx = int(t.nodes[idx].right)
		}

		if idx < 0 || idx >= len(t.nodes) {
			return 0, ErrCorruptStream
		}
	}
	return uint16(t.nodes[idx].symbol), nil
}
