# Testdata Generators — Sources and References

All generated test archives in `testdata/` are built deterministically from the repository's existing sample files (`TESTDAT1.TXT`, `TESTDAT2.TXT`, `TESTDAT3.TXT`). No external or copyrighted files are used.

---

## 1. SEA ARC Generators

| Script | Output File | Method | Reference |
| :--- | :--- | :--- | :--- |
| `make_old.go` | `ARC_OLD.ARC` | Method 1 (Old Stored) | `arc.h` / `arcs.h` (Thom Henderson / Howard Chu `arc` 5.21p) |
| `make_pack.go` | `ARC_PACK.ARC` | Method 3 (Packed / RLE90) | `arcpack.c` (`DLE = 0x90` Non-Repeat Compression) |
| `make_squeeze.go` | `ARC_SQUE.ARC` | Method 4 (Squeezed) | `arcsq.c` / Dick Greenlaw `SQ` (Huffman + RLE90) |
| `make_crunched5.go` | `ARC_CRU5.ARC` | Method 5 (Old Crunched) | `arclzw.c` (12-bit static LZW, middle-square hash) |
| `make_crunched6.go` | `ARC_CRU6.ARC` | Method 6 (Old Crunched + RLE) | `arclzw.c` (12-bit static LZW + RLE90) |
| `make_crunched7.go` | `ARC_CRU7.ARC` | Method 7 (Crunched New Hash) | `arclzw.c` (12-bit LZW with Ross Williams' `15073` hash + RLE90) |

All generated ARC archives were verified against `/usr/bin/arc v` and `/usr/bin/arc t` (Howard Chu's Unix port of SEA ARC).

---

## 2. Library Utility (`.LBR`) Generator

| Script | Output File | Format | Reference |
| :--- | :--- | :--- | :--- |
| `make_lbr.go` | `TEST.LBR` | LUDEF5 `.LBR` | Gary P. Novosielski (`LUDEF5.DOC`, Aug 1984); CCITT/XMODEM CRC-16 (`0x1021`) |

Verified against `/usr/bin/lsar` and `/usr/bin/unar` (The Unarchiver).

---

## 3. LHA / LZH Generators

| Script | Output Files | Methods | Reference |
| :--- | :--- | :--- | :--- |
| `make_lha.go` | `LH4.LZH`, `LH6.LZH`, `LH7.LZH`, `LZ5.LZH` | `-lh4-`, `-lh6-`, `-lh7-`, `-lz5-` | Haruhiko Okumura `ar002` (LZSS + canonical Huffman) & Kazuhiko Miki `LArc` (`-lz5-`) |

All generated LZH archives were verified against `/usr/bin/lha t` (Lhasa v0.6.0) and `/usr/bin/unar`.
