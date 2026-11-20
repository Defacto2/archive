// The EXE support in this file is derived from the PKLITE module of Deark by
// Jason Summers, https://github.com/jsummers/deark. See the copyright notice
// and MIT license in pklite.go.

package pklite

import (
	"bytes"
	"errors"
	"fmt"
)

var ErrUnsupported = errors.New("pklite: unsupported variant")

const (
	paragraph  = 16
	pageSize   = 512
	exeHdrMin  = 28
	epBytesLen = 1000 // the most bytes read from the entry point
)

// exeHeader is the MZ header of a DOS executable.
type exeHeader struct {
	lastPage, pages, relocs, hdrParas uint16
	minAlloc, maxAlloc                uint16
	ss, sp, csum, ip, cs              uint16
	relocPos                          uint16

	startOfCode int // file offset of the code
	endOfCode   int // file offset of the end of the code
	entryPoint  int // file offset of the entry point
}

func le16(b []byte, i int) uint16 { return uint16(b[i]) | uint16(b[i+1])<<8 }

func put16(b []byte, v int) []byte { return append(b, byte(v), byte(v>>8)) }

// parseEXE reads the MZ header from the data.
func parseEXE(data []byte) (exeHeader, bool) {
	var h exeHeader
	if len(data) < exeHdrMin || data[0] != 'M' && data[0] != 'Z' || data[1] != 'Z' && data[1] != 'M' {
		return h, false
	}
	h.lastPage, h.pages, h.relocs = le16(data, 2), le16(data, 4), le16(data, 6)
	h.hdrParas, h.minAlloc, h.maxAlloc = le16(data, 8), le16(data, 10), le16(data, 12)
	h.ss, h.sp, h.csum = le16(data, 14), le16(data, 16), le16(data, 18)
	h.ip, h.cs, h.relocPos = le16(data, 20), le16(data, 22), le16(data, 24)
	h.startOfCode = int(h.hdrParas) * paragraph
	h.endOfCode = int(h.pages) * pageSize
	if h.lastPage > 0 && h.pages > 0 {
		h.endOfCode = (int(h.pages)-1)*pageSize + int(h.lastPage)
	}
	h.entryPoint = h.startOfCode + int(int16(h.cs))*paragraph + int(h.ip) //nolint:gosec
	return h, true
}

// IsEXE reports whether data looks like a PKLITE compressed EXE file.
func IsEXE(data []byte) bool {
	_, ok := parseEXE(data)
	return ok && len(data) >= 0x24 && bytes.Equal(data[0x1e:0x24], []byte("PKLITE"))
}

// find returns the index of the pattern in mem[from:to] or -1.
// The ? byte in the pattern matches any byte.
func find(mem []byte, from, to int, pat string) int {
	from = max(from, 0)
	to = min(to, len(mem))
	for i := from; i+len(pat) <= to; i++ {
		ok := true
		for j := range len(pat) {
			if pat[j] != '?' && mem[i+j] != pat[j] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

// exeParams are the compression parameters found in the executable.
type exeParams struct {
	dataPos int // file offset of the compressed data
	large   bool
	extra   bool
}

// analyze finds the offset of the compressed data and the compression type.
//
// The decompressor is a small program with the locations and sizes of its parts
// stored as machine code operands, so known byte patterns are searched for.
// Unscrambled v1.00 to v1.15 decompressors are supported.
func analyze(data []byte, h exeHeader) (exeParams, error) {
	var p exeParams
	if h.entryPoint < 0 || h.entryPoint >= len(data) {
		return p, ErrNotPKLITE
	}
	ep := data[h.entryPoint:min(len(data), h.entryPoint+epBytesLen)]
	// ipToEP converts an offset in the decompressor segment into an entry point offset.
	ipToEP := func(ip int) int { return max(h.startOfCode+ip-0x100-h.entryPoint, 0) }

	// copier: mov cx,N; xor di,di; push di; mov si,DECOMPRESSOR; cld; rep movsw; retf
	c := find(ep, 0, 120, "\xb9??\x33\xff\x57\xbe??\xfc\xf3\xa5\xcb")
	if c < 0 {
		return p, fmt.Errorf("%w: copier not found", ErrUnsupported)
	}
	decomp := ipToEP(int(le16(ep, c+7)))
	if decomp+8 > len(ep) {
		return p, fmt.Errorf("%w: decompressor not found", ErrUnsupported)
	}
	var n int
	switch {
	case find(ep, decomp, decomp+6, "\xfd\x8c\xdb\x53\x83\xc3") == decomp:
		n = int(ep[decomp+6]) * paragraph
	case find(ep, decomp, decomp+6, "\xfd\x8c\xdb\x53\x81\xc3") == decomp:
		n = int(le16(ep, decomp+6)) * paragraph
	default:
		return p, fmt.Errorf("%w: decompressor not recognized", ErrUnsupported)
	}
	p.dataPos = h.entryPoint + ipToEP(n)
	end := p.dataPos - h.entryPoint // the approximate end of the decompressor
	if p.dataPos <= 0 || p.dataPos >= len(data) || end > len(ep) {
		return p, fmt.Errorf("%w: compressed data position", ErrUnsupported)
	}

	// the extra compression obfuscates the literal bytes with the bit count.
	switch i := find(ep, decomp, end, "\xad\x95\xb2\x10\x72?\xac??\xaa\xd1\xed\x4a\x74"); {
	case find(ep, decomp, end, "\xad\x95\xb2\x10\x72\x08\xa4\xd1\xed\x4a\x74") >= 0:
		p.extra = false
	case i >= 0 && ep[i+5] == 0x0b && ep[i+7] == 0x32 && ep[i+8] == 0xc2:
		p.extra = true
	default:
		return p, fmt.Errorf("%w: compression variant", ErrUnsupported)
	}
	const sig = "\x01\x02\x00\x00\x03\x04\x05\x06\x00\x00\x00\x00\x00\x00\x00\x00\x07\x08\x09\x0a\x0b"
	i := find(ep, end-60, end, sig)
	if i <= 0 {
		return p, fmt.Errorf("%w: v1.20 compression", ErrUnsupported)
	}
	switch ep[i-1] {
	case 0x09:
		p.large = false
	case 0x18:
		p.large = true
	default:
		return p, fmt.Errorf("%w: compression type", ErrUnsupported)
	}
	return p, nil
}

// decompressEXE returns the original EXE from the PKLITE compressed EXE.
func decompressEXE(data []byte) ([]byte, error) {
	h, ok := parseEXE(data)
	if !ok || !IsEXE(data) {
		return nil, ErrNotPKLITE
	}
	if h.endOfCode > len(data) || h.endOfCode < h.startOfCode {
		return nil, fmt.Errorf("%w: bad file size", ErrCorrupt)
	}
	p, err := analyze(data, h)
	if err != nil {
		return nil, err
	}
	code, pos, err := decode(data[:h.endOfCode], p.dataPos, options{large: p.large, extra: p.extra})
	if err != nil {
		return nil, err
	}

	// the compressed relocation table, then the footer of the registers
	relocs, pos, err := readRelocs(data[:max(h.endOfCode-8, 0)], pos, p.extra)
	if err != nil {
		return nil, err
	}
	if h.endOfCode-pos < 8 {
		return nil, fmt.Errorf("%w: missing footer", ErrCorrupt)
	}
	foot := data[pos:]
	reg := regs{ss: le16(foot, 0), sp: le16(foot, 2), cs: le16(foot, 4), ip: le16(foot, 6)}

	var hdr []byte
	if !p.extra { // the extra compression does not store the original header
		hdr = originalHeader(data, h, reg, len(relocs)/4)
	}
	if hdr == nil {
		hdr = reconstructHeader(data, h, reg, len(relocs)/4, code)
	}
	g, _ := parseEXE(hdr)
	relocPos := max(int(g.relocPos), exeHdrMin)

	out := make([]byte, 0, g.startOfCode+len(code))
	out = append(out, hdr[:min(len(hdr), relocPos)]...)
	out = append(out, make([]byte, max(relocPos-len(out), 0))...)
	out = append(out, relocs...)
	out = append(out, make([]byte, max(g.startOfCode-len(out), 0))...)
	out = append(out, code...)
	if over := len(data) - h.endOfCode; over > 0 && len(out) == g.endOfCode {
		out = append(out, data[h.endOfCode:]...)
	}
	return out, nil
}

type regs struct{ ss, sp, cs, ip uint16 }

// readRelocs reads the compressed relocation table and returns
// it as a MZ relocation table, with the position following the table.
// The 'long' table is used by the extra compression.
func readRelocs(data []byte, pos int, long bool) ([]byte, int, error) {
	const maxRelocs = 65535
	if long {
		return readLongRelocs(data, pos)
	}
	var table []byte
	for {
		if pos >= len(data) {
			return nil, 0, fmt.Errorf("%w: relocation table", ErrTruncated)
		}
		count := int(data[pos])
		pos++
		if count == 0 {
			return table, pos, nil
		}
		if len(table)/4+count > maxRelocs || pos+2+count*2 > len(data) {
			return nil, 0, fmt.Errorf("%w: relocation table", ErrCorrupt)
		}
		seg := le16(data, pos)
		pos += 2
		for range count {
			table = append(table, data[pos], data[pos+1], byte(seg), byte(seg>>8))
			pos += 2
		}
	}
}

// originalHeader returns a copy of the original EXE header, which is stored in the
// compressed file after the relocation table. Nil is returned if it is missing or bad.
func originalHeader(data []byte, h exeHeader, reg regs, relocs int) []byte {
	pos := int(h.relocPos) + 4*int(h.relocs)
	n := h.startOfCode - pos
	if n < 26 || pos+n > len(data) {
		return nil
	}
	if rp := int(le16(data, pos+22)); rp >= exeHdrMin && rp < 2+n {
		n = rp - 2
	}
	if le16(data, pos) > 511 || le16(data, pos+2) == 0 {
		return nil
	}
	hdr := append([]byte{'M', 'Z'}, data[pos:pos+n]...)
	if len(hdr) < exeHdrMin {
		return nil
	}
	g, _ := parseEXE(hdr)
	if g.ss != reg.ss || g.sp != reg.sp || g.cs != reg.cs || g.ip != reg.ip || int(g.relocs) != relocs {
		return nil
	}
	return hdr
}

// reconstructHeader builds a header when the original is not stored.
// The memory allocation values are estimated from the compressed file.
func reconstructHeader(data []byte, h exeHeader, reg regs, relocs int, code []byte) []byte {
	minMem, maxMem := int(h.minAlloc), int(h.maxAlloc)
	if maxMem == 0 {
		maxMem = 0xffff
	}
	// The decompressor loads the size of the program in paragraphs at the entry point.
	if ep := h.entryPoint; ep >= 0 && ep+3 <= len(data) {
		i := ep
		if data[i] == 0x50 {
			i++
		}
		if data[i] == 0xb8 && i+3 <= len(data) {
			if n := int(le16(data, i+1))<<4 + 0x100 - len(code); n >= 0 {
				minMem = (n + 0xf) >> 4
			}
		}
	}
	maxMem = max(maxMem, minMem)

	start := (exeHdrMin + relocs*4 + paragraph - 1) / paragraph * paragraph
	end := start + len(code)
	hdr := []byte{'M', 'Z'}
	for _, v := range []int{
		end % pageSize, (end + pageSize - 1) / pageSize, relocs, start / paragraph,
		minMem, maxMem, int(reg.ss), int(reg.sp), 0, int(reg.ip), int(reg.cs), exeHdrMin, 0,
	} {
		hdr = put16(hdr, v)
	}
	return hdr
}

func readLongRelocs(data []byte, pos int) ([]byte, int, error) {
	const maxRelocs = 65535
	var table []byte
	seg := 0
	for {
		if pos+2 > len(data) {
			return nil, 0, fmt.Errorf("%w: relocation table", ErrTruncated)
		}
		count := int(le16(data, pos))
		pos += 2
		if count == 0xffff {
			return table, pos, nil
		}
		if seg > 0xffff || len(table)/4+count > maxRelocs || pos+count*2 > len(data) {
			return nil, 0, fmt.Errorf("%w: relocation table", ErrCorrupt)
		}
		for range count {
			table = append(table, data[pos], data[pos+1], byte(seg), byte(seg>>8))
			pos += 2
		}
		seg += 0x0fff
	}
}
