// The EXE support in this file is derived from the PKLITE module of Deark by
// Jason Summers, https://github.com/jsummers/deark. See the copyright notice
// and MIT license in pklite.go.

package pklite

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

var ErrUnsupported = errors.New("pklite: unsupported variant")

const (
	paragraph  = 16
	pageSize   = 512
	headerSize = 28    // "MZ" signature header minimum length
	mzSize     = 2     // "MZ" signature bytes omitted from stored headers
	entrypoint = 1000  // the most bytes read from the entry point
	pspSize    = 0x100 // DOS Program Segment Prefix (256 bytes)
	footerSize = 8     // saved registers (SS, SP, CS, IP)
	uint16Mask = 0xffff

	// PKLITE header signature.
	sigOffset = 0x1e // offset of "PKLITE" in MZ header
	sigSize   = 6

	// Decompressor stub detection.
	cpSearch   = 120  // max bytes from entry point to search for copier
	cpDecomp   = 7    // offset of decompressor IP in copier pattern
	decompSize = 6    // length of std/mov/push/add-bx prefix
	largeSig   = 60   // tail bytes before end of stub to search for table
	modeSmall  = 0x09 // small-compression marker byte
	modeLarge  = 0x18 // large-compression marker byte
	opPushAX   = 0x50 // 8086 "push ax" opcode
	opMovAX    = 0xb8 // 8086 "mov ax, imm16" opcode

	// Saved register offsets in footer.
	footSS = 0
	footSP = 2
	footCS = 4
	footIP = 6

	// Long relocation table markers.
	relocEntrySize = 4
	relocStop      = 0xffff
	relocStep      = 0x0fff
)

// MZ header field byte offsets.
const (
	hdrLastPage = 2
	hdrPages    = 4
	hdrRelocs   = 6
	hdrParas    = 8
	hdrMinAlloc = 10
	hdrMaxAlloc = 12
	hdrSS       = 14
	hdrSP       = 16
	hdrCsum     = 18
	hdrIP       = 20
	hdrCS       = 22
	hdrRelocPos = 24
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

func put16(b []byte, v int) []byte {
	return binary.LittleEndian.AppendUint16(b, uint16(v&uint16Mask))
}

// parseEXE reads the MZ header from the data.
func parseEXE(data []byte) (exeHeader, bool) {
	var h exeHeader
	if len(data) < headerSize ||
		data[0] != 'M' && data[0] != 'Z' ||
		data[1] != 'Z' && data[1] != 'M' {
		return h, false
	}

	h.lastPage, h.pages, h.relocs =
		le16(data, hdrLastPage), le16(data, hdrPages), le16(data, hdrRelocs)
	h.hdrParas, h.minAlloc, h.maxAlloc =
		le16(data, hdrParas), le16(data, hdrMinAlloc), le16(data, hdrMaxAlloc)
	h.ss, h.sp, h.csum =
		le16(data, hdrSS), le16(data, hdrSP), le16(data, hdrCsum)
	h.ip, h.cs, h.relocPos =
		le16(data, hdrIP), le16(data, hdrCS), le16(data, hdrRelocPos)
	h.startOfCode = int(h.hdrParas) * paragraph
	h.endOfCode = int(h.pages) * pageSize
	if h.lastPage > 0 && h.pages > 0 {
		h.endOfCode = (int(h.pages)-1)*pageSize + int(h.lastPage)
	}

	cs := int(h.cs)
	if cs > math.MaxInt16 {
		cs -= math.MaxUint16 + 1
	}
	h.entryPoint = h.startOfCode +
		cs*paragraph +
		int(h.ip)

	return h, true
}

// IsEXE reports whether data looks like a PKLITE compressed EXE file.
func IsEXE(data []byte) bool {
	_, ok := parseEXE(data)
	return ok && len(data) >= sigOffset+sigSize &&
		bytes.Equal(data[sigOffset:sigOffset+sigSize], []byte("PKLITE"))
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
	const format = "analyze compressed data %s: %w"
	var p exeParams
	if h.entryPoint < 0 || h.entryPoint >= len(data) {
		return p, ErrNotPKLITE
	}

	ep := data[h.entryPoint:min(len(data), h.entryPoint+entrypoint)]
	// ipToEP converts an offset in the decompressor segment into an entry point offset.
	ipToEP := func(ip int) int { return max(h.startOfCode+ip-pspSize-h.entryPoint, 0) }

	decomp, dataIP, err := findDecompressor(ep, ipToEP)
	if err != nil {
		return p, err
	}

	p.dataPos = h.entryPoint + ipToEP(dataIP)
	end := p.dataPos - h.entryPoint // the approximate end of the decompressor
	if p.dataPos <= 0 || p.dataPos >= len(data) || end > len(ep) {
		return p, fmt.Errorf(format, "position", ErrUnsupported)
	}

	p.extra, err = detectExtra(ep, decomp, end)
	if err != nil {
		return p, err
	}

	p.large, err = detectLarge(ep, end)
	if err != nil {
		return p, err
	}
	return p, nil
}

func findDecompressor(ep []byte, ipToEP func(int) int) (int, int, error) {
	const format = "find decompressor %s: %w"
	// copier: mov cx,N; xor di,di; push di; mov si,DECOMPRESSOR; cld; rep movsw; retf
	c := find(ep, 0, cpSearch, "\xb9??\x33\xff\x57\xbe??\xfc\xf3\xa5\xcb")
	if c < 0 {
		return 0, 0, fmt.Errorf(format, "copier not found", ErrUnsupported)
	}

	decomp := ipToEP(int(le16(ep, c+cpDecomp)))
	if decomp+8 > len(ep) {
		return 0, 0, fmt.Errorf(format, "decompressor not found", ErrUnsupported)
	}

	switch {
	case find(ep, decomp, decomp+decompSize, "\xfd\x8c\xdb\x53\x83\xc3") == decomp:
		return decomp, int(ep[decomp+decompSize]) * paragraph, nil
	case find(ep, decomp, decomp+decompSize, "\xfd\x8c\xdb\x53\x81\xc3") == decomp:
		return decomp, int(le16(ep, decomp+decompSize)) * paragraph, nil
	default:
		return 0, 0, fmt.Errorf(format, "decompressor not recognized", ErrUnsupported)
	}
}

func detectExtra(ep []byte, decomp, end int) (bool, error) {
	const format = "detect extract %s: %w"
	// the extra compression obfuscates the literal bytes with the bit count.
	if find(ep, decomp, end, "\xad\x95\xb2\x10\x72\x08\xa4\xd1\xed\x4a\x74") >= 0 {
		return false, nil
	}

	i := find(ep, decomp, end, "\xad\x95\xb2\x10\x72?\xac??\xaa\xd1\xed\x4a\x74")
	if i >= 0 && ep[i+5] == 0x0b && ep[i+7] == 0x32 && ep[i+8] == 0xc2 {
		return true, nil
	}
	return false, fmt.Errorf(format, "compression variant", ErrUnsupported)
}

func detectLarge(ep []byte, end int) (bool, error) {
	const format = "detect large %s: %w"
	const sig = "\x01\x02\x00\x00\x03\x04\x05\x06\x00\x00\x00\x00\x00\x00\x00\x00\x07\x08\x09\x0a\x0b"
	i := find(ep, end-largeSig, end, sig)
	if i <= 0 {
		return false, fmt.Errorf(format, "v1.20 compression", ErrUnsupported)
	}

	switch ep[i-1] {
	case modeSmall:
		return false, nil
	case modeLarge:
		return true, nil
	default:
		return false, fmt.Errorf(format, "compression type", ErrUnsupported)
	}
}

// decompressEXE returns the original EXE from the PKLITE compressed EXE.
func decompressEXE(data []byte) ([]byte, error) {
	const format = "decompress exe %s: %w"
	h, ok := parseEXE(data)
	if !ok || !IsEXE(data) {
		return nil, ErrNotPKLITE
	}

	if h.endOfCode > len(data) || h.endOfCode < h.startOfCode {
		return nil, fmt.Errorf(format, "bad file size", ErrCorrupt)
	}

	p, err := analyze(data, h)
	if err != nil {
		return nil, err
	}

	code, pos, err := decode(data[:h.endOfCode], p.dataPos,
		options{large: p.large, extra: p.extra, prehistory: false})
	if err != nil {
		return nil, err
	}

	// the compressed relocation table, then the footer of the registers
	relocs, pos, err := readRelocs(data[:max(h.endOfCode-footerSize, 0)], pos, p.extra)
	if err != nil {
		return nil, err
	}

	if h.endOfCode-pos < footerSize {
		return nil, fmt.Errorf(format, "missing footer", ErrCorrupt)
	}

	foot := data[pos:]
	reg := regs{
		ss: le16(foot, footSS),
		sp: le16(foot, footSP),
		cs: le16(foot, footCS),
		ip: le16(foot, footIP),
	}

	var hdr []byte
	if !p.extra { // the extra compression does not store the original header
		hdr = originalHeader(data, h, reg, len(relocs)/relocEntrySize)
	}
	if hdr == nil {
		hdr = reconstructHeader(data, h, reg, len(relocs)/relocEntrySize, code)
	}
	g, _ := parseEXE(hdr)
	relocPos := max(int(g.relocPos), headerSize)

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
	const format = "compressed relocation table: %w"
	const maxRelocs = 65535
	if long {
		return readLongRelocs(data, pos)
	}

	var table []byte
	for {
		if pos >= len(data) {
			return nil, 0, fmt.Errorf(format, ErrTruncated)
		}
		count := int(data[pos])
		pos++
		if count == 0 {
			return table, pos, nil
		}
		if len(table)/4+count > maxRelocs || pos+2+count*2 > len(data) {
			return nil, 0, fmt.Errorf(format, ErrCorrupt)
		}
		seg := le16(data, pos)
		pos += 2
		for range count {
			table = binary.LittleEndian.AppendUint16(append(table, data[pos], data[pos+1]), seg)
			pos += 2
		}
	}
}

// originalHeader returns a copy of the original EXE header, which is stored in the
// compressed file after the relocation table. Nil is returned if it is missing or bad.
func originalHeader(data []byte, h exeHeader, reg regs, relocs int) []byte {
	pos := int(h.relocPos) + relocEntrySize*int(h.relocs)
	n := h.startOfCode - pos
	if n < headerSize-mzSize || pos+n > len(data) {
		return nil
	}

	if rp := int(le16(data, pos+hdrRelocPos-mzSize)); rp >= headerSize && rp < mzSize+n {
		n = rp - mzSize
	}

	if le16(data, pos) >= pageSize || le16(data, pos+hdrPages-mzSize) == 0 {
		return nil
	}

	hdr := append([]byte{'M', 'Z'}, data[pos:pos+n]...)
	if len(hdr) < headerSize {
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
		if data[i] == opPushAX {
			i++
		}
		if data[i] == opMovAX && i+3 <= len(data) {
			if n := int(le16(data, i+1))*paragraph + pspSize - len(code); n >= 0 {
				minMem = (n + paragraph - 1) / paragraph
			}
		}
	}
	maxMem = max(maxMem, minMem)

	start := (headerSize + relocs*4 + paragraph - 1) / paragraph * paragraph
	end := start + len(code)
	hdr := []byte{'M', 'Z'}
	for _, v := range []int{
		end % pageSize, (end + pageSize - 1) / pageSize, relocs, start / paragraph,
		minMem, maxMem, int(reg.ss), int(reg.sp), 0, int(reg.ip), int(reg.cs), headerSize, 0,
	} {
		hdr = put16(hdr, v)
	}
	return hdr
}

func readLongRelocs(data []byte, pos int) ([]byte, int, error) {
	const format = "long relocation table: %w"
	const maxRelocs = 65535
	var table []byte
	seg := 0
	for {
		if pos+2 > len(data) {
			return nil, 0, fmt.Errorf(format, ErrTruncated)
		}

		count := int(le16(data, pos))
		pos += 2
		if count == relocStop {
			return table, pos, nil
		}

		if seg > 0xffff || len(table)/4+count > maxRelocs || pos+count*2 > len(data) {
			return nil, 0, fmt.Errorf(format, ErrCorrupt)
		}

		for range count {
			table = binary.LittleEndian.AppendUint16(append(table, data[pos], data[pos+1]), uint16(seg&uint16Mask))
			pos += 2
		}

		seg += relocStep
	}
}
