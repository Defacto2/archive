# Sources and References

This lists the codebases, specifications, and references used by Gemini to generate `github.com/Defacto2/archive/arc`.

---

## 1. Codebases

### Howard Chu's Unix Port of SEA ARC (Version 5.21p)

- **Repository Mirror:** [https://github.com/hyc/arc](https://github.com/hyc/arc)
- **Original Authors:** Thom Henderson (System Enhancement Associates, 1985–1986); Unix port by Howard Chu (1987–2010).
- **Referenced Source Files:**
  - `arc.h` & `arcs.h`: Header layouts (23-byte Method 1 and 27-byte Method 2–9), marker byte (`0x1A`), and method definitions.
  - `arcunp.c`: Unpack dispatcher and Non-Repeat Compression (NCR / RLE90) state machine (`putb_ncr`, `DLE = 0x90`, run count decoding).
  - `arcusq.c`: Huffman unsqueeze decoding algorithm (`init_usq`, `getb_usq`), 16-bit node count prefix, binary interior decode tree with negative leaf indices, and `SPEOF = 256` token handling.
  - `arclzw.c`:
    - Dynamic LZW decompressor (`decomp()`) for Method 8 (Crunched) and Method 9 (Squashed), code sizes 9–13 bits, block clearing flags, and buffer alignment.
    - Static 12-bit fixed hash LZW decompressor (`init_ucr()`, `getb_ucr()`, `upd_tab()`, `oldh()`, `newh()`, `GOCODE()` 12-bit nybble unpacking) for Methods 5, 6, and 7.
  - `arcpack.c` & `arcsq.c`: Huffman frequency tree construction and interior node serialization (`head_sq()`).

#### AUTHORS

>     Original MSDOS program by Thom Henderson
>     COPYRIGHT(C) 1985-87 by System Enhancement Associates; ALL RIGHTS RESERVED
>
>     Original  Lempel-Zev code derived from compress 4.0.  Modified to support Squashing by Dan Lanciani (ddl@harvard.edu)
>     Ported from MSDOS by Howard Chu, with help from John Gilmore (hoptoad!gnu), James Turner (daisy!turner) and others.

### Russell Marks' nomarch (Version 1.4)

- **Distribution:** [Debian Sources: nomarch](https://sources.debian.org/src/nomarch/1.4-5/)
- **Author:** Russell Marks (2001–2006).
- **License:** GNU General Public License (GPLv2+).
- **Referenced For:** Verification of clean Unix extraction routines (`readhuff.c`, `readlzw.c`, `readrle.c`).

---

## 2. References

### CP/M Squeeze and Unsqueeze (USQ)

- **Authors:** Richard Greenlaw (1981), Chuck Forsberg (1981); adapted to C86 by Robert J. Beilstein.
- **Status:** Public Domain.
- **Referenced For:** Original Huffman tree + RLE compression design used across CP/M and early MS-DOS archivers.

### CP/M Crunch and Uncrunch (UNCRUNCH)

- **Author:** Steven Greenberg (1985–1986).
- **Status:** Public Domain.
- **Referenced For:** Fixed 12-bit dictionary LZW implementation and collision handling with middle-square hashing.

### Terry A. Welch: "A Technique for High-Performance Data Compression"

- **Citation:** IEEE Computer, Vol. 17, No. 6 (June 1984), pp. 8–19.
- **Referenced For:** Foundations of the LZW dictionary string-table algorithms utilized by Unix `compress` and SEA ARC dynamic crunching.

### Donald E. Knuth: The Art of Computer Programming, Vol. 3 (Sorting and Searching)

- **Section:** Section 6.4 (Hashing and Secondary Probing).
- **Referenced For:** Double hashing and secondary probe steps used in ARC's string dictionary.
