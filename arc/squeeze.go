package arc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const speof = 256 // Special end of file token for Huffman squeeze

// Unsqueeze decompresses data compressed with the following SEA ARC method:
//   - Method 4: Compression of repeated characters plus Huffman SQueezing
func Unsqueeze(r io.Reader) ([]byte, error) { //nolint:funlen
	var count int16
	err := binary.Read(r, binary.LittleEndian, &count)
	if err != nil {
		return nil, fmt.Errorf("read count: %w", err)
	}
	if count < 0 || count > 257 {
		return nil, fmt.Errorf("node count %d: %w", count, ErrInvalidTree)
	}

	type node struct {
		child [2]int16
	}
	nodes := make([]node, count)
	for i := range nodes {
		err := binary.Read(r, binary.LittleEndian, &nodes[i].child[0])
		if err != nil {
			return nil, fmt.Errorf("read left child: %w", err)
		}
		err = binary.Read(r, binary.LittleEndian, &nodes[i].child[1])
		if err != nil {
			return nil, fmt.Errorf("read right child: %w", err)
		}
	}

	const maxBits = 7
	var buf bytes.Buffer
	var chr byte
	bpos := 99

	for {
		curNode := int16(0)
		for curNode >= 0 {
			bpos++
			if bpos > maxBits {
				var b [1]byte
				_, err := io.ReadFull(r, b[:])
				if err != nil {
					if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
						return nil, fmt.Errorf("found an end of file marker before the special end of file token: %w", ErrInvalidTree)
					}
					return nil, fmt.Errorf("unsqueeze read full: %w", err)
				}
				chr = b[0]
				bpos = 0
			}
			bit := (chr >> bpos) & 1
			if int(curNode) >= len(nodes) {
				return nil, fmt.Errorf("node index %d is out of bounds: %w", curNode, ErrInvalidTree)
			}
			curNode = nodes[curNode].child[bit]
		}

		val := -(curNode + 1)
		if val == speof {
			break
		}
		buf.WriteByte(byte(val))
	}

	return UnpackRLE(buf.Bytes()), nil
}
