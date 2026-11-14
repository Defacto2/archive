package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func crc16LHA(data []byte) uint16 {
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

type bitWriter struct {
	buf bytes.Buffer
	acc uint32
	n   int
}

func (bw *bitWriter) writeBits(val uint32, bits int) {
	bw.acc = (bw.acc << bits) | (val & ((1 << bits) - 1))
	bw.n += bits
	for bw.n >= 8 {
		bw.n -= 8
		bw.buf.WriteByte(byte(bw.acc >> bw.n))
	}
}

func (bw *bitWriter) flush() []byte {
	if bw.n > 0 {
		bw.buf.WriteByte(byte(bw.acc << (8 - bw.n)))
		bw.n = 0
	}
	return bw.buf.Bytes()
}

// buildCanonicalHuffman assigns canonical Huffman code lengths (max 16 bits)
// and codes for a slice of symbol frequencies.
func buildCanonicalHuffman(freq []int) (lengths []int, codes []uint32) {
	n := len(freq)
	lengths = make([]int, n)
	codes = make([]uint32, n)

	// Collect non-zero symbols
	var active []int
	for i, f := range freq {
		if f > 0 {
			active = append(active, i)
		}
	}
	if len(active) <= 1 {
		return lengths, codes
	}

	// Build Huffman tree using simple O(k^2) merging
	type hNode struct {
		freq  int
		sym   int
		left  *hNode
		right *hNode
	}
	nodes := make([]*hNode, len(active))
	for i, s := range active {
		nodes[i] = &hNode{freq: freq[s], sym: s}
	}

	for len(nodes) > 1 {
		// Find two smallest
		m1, m2 := 0, 1
		if nodes[m2].freq < nodes[m1].freq {
			m1, m2 = m2, m1
		}
		for i := 2; i < len(nodes); i++ {
			if nodes[i].freq < nodes[m1].freq {
				m2 = m1
				m1 = i
			} else if nodes[i].freq < nodes[m2].freq {
				m2 = i
			}
		}
		parent := &hNode{
			freq:  nodes[m1].freq + nodes[m2].freq,
			sym:   -1,
			left:  nodes[m1],
			right: nodes[m2],
		}
		if m1 < m2 {
			nodes = append(nodes[:m2], nodes[m2+1:]...)
			nodes[m1] = parent
		} else {
			nodes = append(nodes[:m1], nodes[m1+1:]...)
			nodes[m2] = parent
		}
	}

	var walk func(node *hNode, depth int)
	walk = func(node *hNode, depth int) {
		if node.sym >= 0 {
			if depth > 16 {
				depth = 16
			}
			lengths[node.sym] = depth
			return
		}
		walk(node.left, depth+1)
		walk(node.right, depth+1)
	}
	walk(nodes[0], 0)

	// Clamp & adjust using Kraft inequality if any depth hit 16
	// For typical text blocks (< 16384 tokens), depth rarely exceeds 15.
	var blCount [17]int
	for _, l := range lengths {
		if l > 0 {
			blCount[l]++
		}
	}
	var nextCode [17]uint32
	code := uint32(0)
	for bits := 1; bits <= 16; bits++ {
		code = (code + uint32(blCount[bits-1])) << 1
		nextCode[bits] = code
	}
	for i := 0; i < n; i++ {
		l := lengths[i]
		if l > 0 {
			codes[i] = nextCode[l]
			nextCode[l]++
		}
	}
	return lengths, codes
}

type token struct {
	isMatch bool
	lit     byte
	length  int
	offset  int // 0-based distance - 1
}

func lzssTokenize(data []byte, dictBits int) []token {
	winSize := 1 << dictBits
	const minMatch = 3
	const maxMatch = 256

	var tokens []token
	pos := 0
	for pos < len(data) {
		bestLen := 0
		bestOff := 0
		limit := pos - winSize
		if limit < 0 {
			limit = 0
		}
		// Simple backward search (fast enough for small testdata)
		for cand := pos - 1; cand >= limit && pos-cand <= 4096; cand-- {
			if data[cand] != data[pos] {
				continue
			}
			mLen := 1
			for mLen < maxMatch && pos+mLen < len(data) && data[cand+mLen] == data[pos+mLen] {
				mLen++
			}
			if mLen >= minMatch && mLen > bestLen {
				bestLen = mLen
				bestOff = pos - cand - 1
				if bestLen == maxMatch {
					break
				}
			}
		}
		if bestLen >= minMatch {
			tokens = append(tokens, token{isMatch: true, length: bestLen, offset: bestOff})
			pos += bestLen
		} else {
			tokens = append(tokens, token{lit: data[pos]})
			pos++
		}
	}
	return tokens
}

func bitLen(v int) int {
	n := 0
	for v > 0 {
		n++
		v >>= 1
	}
	return n
}

func encodeLH(data []byte, dictBits, np, pbit int) []byte {
	tokens := lzssTokenize(data, dictBits)
	var bw bitWriter

	const blockMax = 16384
	for start := 0; start < len(tokens); start += blockMax {
		end := start + blockMax
		if end > len(tokens) {
			end = len(tokens)
		}
		blk := tokens[start:end]

		// Write block size (16 bits)
		bw.writeBits(uint32(len(blk)), 16)

		// Compute frequencies for c (510 symbols) and p (np symbols)
		cFreq := make([]int, 510)
		pFreq := make([]int, np)
		for _, t := range blk {
			if t.isMatch {
				cFreq[256+t.length-3]++
				pFreq[bitLen(t.offset)]++
			} else {
				cFreq[t.lit]++
			}
		}

		cLen, cCodes := buildCanonicalHuffman(cFreq)
		pLen, pCodes := buildCanonicalHuffman(pFreq)

		// Build pt tree for encoding cLen (19 symbols: 0..2 are zero runs, 3..18 are lengths 0..15)
		ptFreq := make([]int, 19)
		var ptStream []struct {
			sym   int
			extra uint32
			ebits int
		}
		cCount := 0
		for i, l := range cLen {
			if l > 0 {
				cCount = i + 1
			}
		}
		for i := 0; i < cCount; {
			if cLen[i] == 0 {
				run := 0
				for i+run < cCount && cLen[i+run] == 0 {
					run++
				}
				for run > 0 {
					if run >= 20 {
						chunk := run
						if chunk > 531 {
							chunk = 531
						}
						ptFreq[2]++
						ptStream = append(ptStream, struct {
							sym   int
							extra uint32
							ebits int
						}{sym: 2, extra: uint32(chunk - 20), ebits: 9})
						run -= chunk
					} else if run >= 3 {
						chunk := run
						if chunk > 18 {
							chunk = 18
						}
						ptFreq[1]++
						ptStream = append(ptStream, struct {
							sym   int
							extra uint32
							ebits int
						}{sym: 1, extra: uint32(chunk - 3), ebits: 4})
						run -= chunk
					} else {
						ptFreq[0]++
						ptStream = append(ptStream, struct {
							sym   int
							extra uint32
							ebits int
						}{sym: 0})
						run--
					}
				}
				for k := 0; k < cCount && i+k < cCount && cLen[i+k] == 0; k++ {
				}
				// Advance i by total zeros consumed
				for i < cCount && cLen[i] == 0 {
					i++
				}
			} else {
				sym := cLen[i] + 2
				ptFreq[sym]++
				ptStream = append(ptStream, struct {
					sym   int
					extra uint32
					ebits int
				}{sym: sym})
				i++
			}
		}

		ptLen, ptCodes := buildCanonicalHuffman(ptFreq)

		// Write pt tree (nn=19, nbit=5, i_special=3)
		writeTreeLen(&bw, ptLen, ptFreq, 5, 3)

		// Write cLen using pt tree
		if cCount == 0 {
			bw.writeBits(0, 9)
			singleSym := 0
			for i, f := range cFreq {
				if f > 0 {
					singleSym = i
					break
				}
			}
			bw.writeBits(uint32(singleSym), 9)
		} else {
			// Check if pt tree has only 1 symbol
			ptActive := 0
			for _, f := range ptFreq {
				if f > 0 {
					ptActive++
				}
			}
			bw.writeBits(uint32(cCount), 9)
			for _, item := range ptStream {
				if ptActive > 1 {
					bw.writeBits(ptCodes[item.sym], ptLen[item.sym])
				}
				if item.ebits > 0 {
					bw.writeBits(item.extra, item.ebits)
				}
			}
		}

		// Write p tree (nn=np, nbit=pbit, i_special=-1)
		writeTreeLen(&bw, pLen, pFreq, pbit, -1)

		// Check single-symbol flags
		cActive, pActive := 0, 0
		for _, f := range cFreq {
			if f > 0 {
				cActive++
			}
		}
		for _, f := range pFreq {
			if f > 0 {
				pActive++
			}
		}

		// Emit tokens
		for _, t := range blk {
			if t.isMatch {
				csym := 256 + t.length - 3
				if cActive > 1 {
					bw.writeBits(cCodes[csym], cLen[csym])
				}
				psym := bitLen(t.offset)
				if pActive > 1 {
					bw.writeBits(pCodes[psym], pLen[psym])
				}
				if psym > 1 {
					bw.writeBits(uint32(t.offset), psym-1)
				}
			} else {
				if cActive > 1 {
					bw.writeBits(cCodes[t.lit], cLen[t.lit])
				}
			}
		}
	}

	return bw.flush()
}

func writeTreeLen(bw *bitWriter, lens, freq []int, nbit, iSpecial int) {
	n := len(lens)
	for n > 0 && lens[n-1] == 0 {
		n--
	}
	if n == 0 {
		bw.writeBits(0, nbit)
		single := 0
		for i, f := range freq {
			if f > 0 {
				single = i
				break
			}
		}
		bw.writeBits(uint32(single), nbit)
		return
	}
	bw.writeBits(uint32(n), nbit)
	for i := 0; i < n; i++ {
		l := lens[i]
		if l < 7 {
			bw.writeBits(uint32(l), 3)
		} else {
			bw.writeBits(7, 3)
			for k := 7; k < l; k++ {
				bw.writeBits(1, 1)
			}
			bw.writeBits(0, 1)
		}
		if i+1 == iSpecial {
			zeros := 0
			for zeros < 3 && i+1 < n && lens[i+1] == 0 {
				zeros++
				i++
			}
			bw.writeBits(uint32(zeros), 2)
		}
	}
}

func encodeLZ5(data []byte) []byte {
	const winSize = 4096
	const winMask = winSize - 1
	const threshold = 2 // min match = 3

	ring := make([]byte, winSize)
	for i := range ring {
		ring[i] = ' '
	}
	rPos := winSize - 18

	var out bytes.Buffer
	pos := 0
	for pos < len(data) {
		var flagByte byte
		var chunk bytes.Buffer

		for bit := 0; bit < 8 && pos < len(data); bit++ {
			bestLen := 0
			bestIdx := 0
			maxSearch := 18
			if pos+maxSearch > len(data) {
				maxSearch = len(data) - pos
			}
			if maxSearch > threshold {
				for dist := 1; dist < winSize; dist++ {
					cand := (rPos - dist) & winMask
					if ring[cand] != data[pos] {
						continue
					}
					mLen := 1
					for mLen < maxSearch && mLen < dist && ring[(cand+mLen)&winMask] == data[pos+mLen] {
						mLen++
					}
					if mLen > bestLen {
						bestLen = mLen
						bestIdx = cand
						if bestLen == maxSearch {
							break
						}
					}
				}
			}

			if bestLen > threshold {
				// Match: flag bit = 0
				chunk.WriteByte(byte(bestIdx & 0xFF))
				chunk.WriteByte(byte(((bestIdx >> 4) & 0xF0) | (bestLen - 3)))
				for k := 0; k < bestLen; k++ {
					ring[rPos] = data[pos+k]
					rPos = (rPos + 1) & winMask
				}
				pos += bestLen
			} else {
				// Literal: flag bit = 1
				flagByte |= 1 << bit
				b := data[pos]
				chunk.WriteByte(b)
				ring[rPos] = b
				rPos = (rPos + 1) & winMask
				pos++
			}
		}
		out.WriteByte(flagByte)
		out.Write(chunk.Bytes())
	}
	return out.Bytes()
}

func writeLHA(dst, method string, compress func([]byte) []byte) {
	sources := []string{
		"/home/ben/github/archive/testdata/TESTDAT1.TXT",
		"/home/ben/github/archive/testdata/TESTDAT2.TXT",
		"/home/ben/github/archive/testdata/TESTDAT3.TXT",
	}

	var arc bytes.Buffer
	for _, src := range sources {
		data, err := os.ReadFile(src)
		if err != nil {
			panic(err)
		}
		name := strings.ToLower(filepath.Base(src))
		comp := compress(data)
		crc := crc16LHA(data)

		// Level 0 header: 2 prefix bytes + 22 bytes + filename length
		hdrLen := 24 + len(name)
		hdr := make([]byte, hdrLen)
		hdr[0] = byte(22 + len(name))
		copy(hdr[2:7], method)
		binary.LittleEndian.PutUint32(hdr[7:11], uint32(len(comp)))
		binary.LittleEndian.PutUint32(hdr[11:15], uint32(len(data)))
		binary.LittleEndian.PutUint32(hdr[15:19], 0x5A4E6100) // MS-DOS timestamp
		hdr[19] = 0x20                                        // attribute
		hdr[20] = 0x00                                        // level 0
		hdr[21] = byte(len(name))
		copy(hdr[22:22+len(name)], name)
		binary.LittleEndian.PutUint16(hdr[22+len(name):], crc)

		var sum byte
		for _, b := range hdr[2:] {
			sum += b
		}
		hdr[1] = sum

		arc.Write(hdr)
		arc.Write(comp)
	}
	arc.WriteByte(0) // End of archive

	if err := os.WriteFile(dst, arc.Bytes(), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %s (%s, %d bytes)\n", dst, method, arc.Len())
}

func main() {
	writeLHA("/home/ben/github/archive/testdata/LH4.LZH", "-lh4-", func(b []byte) []byte {
		return encodeLH(b, 12, 14, 4)
	})
	writeLHA("/home/ben/github/archive/testdata/LH6.LZH", "-lh6-", func(b []byte) []byte {
		return encodeLH(b, 15, 16, 5)
	})
	writeLHA("/home/ben/github/archive/testdata/LH7.LZH", "-lh7-", func(b []byte) []byte {
		return encodeLH(b, 16, 17, 5)
	})
	writeLHA("/home/ben/github/archive/testdata/LZ5.LZH", "-lz5-", encodeLZ5)
}
