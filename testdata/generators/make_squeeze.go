package main

import (
	"bytes"
	"container/heap"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const (
	arcMark = 0x1A
	speof   = 256
	dle     = 0x90
)

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

type hNode struct {
	val    int // 0..255 or 256 (SPEOF), or -1 for interior
	weight int
	depth  int
	left   *hNode
	right  *hNode
	index  int // Assigned interior index
}

type nodeHeap []*hNode

func (h nodeHeap) Len() int { return len(h) }
func (h nodeHeap) Less(i, j int) bool {
	if h[i].weight == h[j].weight {
		return h[i].depth < h[j].depth
	}
	return h[i].weight < h[j].weight
}
func (h nodeHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeHeap) Push(x interface{}) { *h = append(*h, x.(*hNode)) }
func (h *nodeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func buildHuffman(rleData []byte) ([]byte, error) {
	counts := make(map[int]int)
	for _, b := range rleData {
		counts[int(b)]++
	}
	counts[speof] = 1 // SPEOF is always present

	h := &nodeHeap{}
	heap.Init(h)
	for val, wt := range counts {
		heap.Push(h, &hNode{val: val, weight: wt, depth: 0})
	}

	for h.Len() > 1 {
		left := heap.Pop(h).(*hNode)
		right := heap.Pop(h).(*hNode)
		parent := &hNode{
			val:    -1,
			weight: left.weight + right.weight,
			depth:  1 + max(left.depth, right.depth),
			left:   left,
			right:  right,
		}
		heap.Push(h, parent)
	}

	root := heap.Pop(h).(*hNode)

	// Collect interior nodes in BFS/preorder so root is index 0
	var interior []*hNode
	var assign func(n *hNode)
	assign = func(n *hNode) {
		if n == nil || n.val >= 0 {
			return
		}
		n.index = len(interior)
		interior = append(interior, n)
		assign(n.left)
		assign(n.right)
	}
	assign(root)

	// Generate bit codes (LSB first)
	type codeInfo struct {
		bits uint32
		len  int
	}
	codes := make(map[int]codeInfo)
	var getCodes func(n *hNode, bits uint32, length int)
	getCodes = func(n *hNode, bits uint32, length int) {
		if n.val >= 0 {
			codes[n.val] = codeInfo{bits: bits, len: length}
			return
		}
		if n.left != nil {
			getCodes(n.left, bits, length+1) // 0 bit
		}
		if n.right != nil {
			getCodes(n.right, bits|(1<<length), length+1) // 1 bit
		}
	}
	getCodes(root, 0, 0)

	var out bytes.Buffer

	// 1. Write numnodes (int16 little endian)
	numNodes := int16(len(interior))
	_ = binary.Write(&out, binary.LittleEndian, numNodes)

	// 2. Write interior nodes
	for _, n := range interior {
		var leftVal, rightVal int16
		if n.left.val >= 0 {
			leftVal = int16(-(n.left.val + 1))
		} else {
			leftVal = int16(n.left.index)
		}

		if n.right.val >= 0 {
			rightVal = int16(-(n.right.val + 1))
		} else {
			rightVal = int16(n.right.index)
		}

		_ = binary.Write(&out, binary.LittleEndian, leftVal)
		_ = binary.Write(&out, binary.LittleEndian, rightVal)
	}

	// 3. Write Huffman encoded bits LSB first
	var bitBuf uint32
	var bitCount int

	writeBits := func(bits uint32, length int) {
		bitBuf |= (bits << bitCount)
		bitCount += length
		for bitCount >= 8 {
			out.WriteByte(byte(bitBuf & 0xFF))
			bitBuf >>= 8
			bitCount -= 8
		}
	}

	for _, b := range rleData {
		c := codes[int(b)]
		writeBits(c.bits, c.len)
	}
	// Write SPEOF
	eofCode := codes[speof]
	writeBits(eofCode.bits, eofCode.len)

	if bitCount > 0 {
		out.WriteByte(byte(bitBuf & 0xFF))
	}

	return out.Bytes(), nil
}

func main() {
	testfile := "/home/ben/github/archive/testdata/TESTDAT2.TXT"
	content, err := os.ReadFile(testfile)
	if err != nil {
		panic(err)
	}

	rleData := packRLE(content)
	huffData, err := buildHuffman(rleData)
	if err != nil {
		panic(err)
	}
	crc := crc16(content)

	var arcBuf bytes.Buffer
	arcBuf.WriteByte(arcMark)
	arcBuf.WriteByte(4) // Method 4: Squeezed

	name := filepath.Base(testfile)
	var nameBytes [13]byte
	copy(nameBytes[:], name)
	arcBuf.Write(nameBytes[:])

	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], uint32(len(huffData)))
	arcBuf.Write(b4[:])

	arcBuf.Write([]byte{0x21, 0x14, 0x48, 0x82}) // Date/Time

	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[:], crc)
	arcBuf.Write(b2[:])

	binary.LittleEndian.PutUint32(b4[:], uint32(len(content)))
	arcBuf.Write(b4[:])

	arcBuf.Write(huffData)
	arcBuf.Write([]byte{arcMark, 0}) // End marker

	dst := "/home/ben/github/archive/testdata/ARC_SQUE.ARC"
	err = os.WriteFile(dst, arcBuf.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %s (orig=%d, squeezed=%d)\n", dst, len(content), len(huffData))
}
