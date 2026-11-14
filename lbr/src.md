# Sources and References

This lists the specifications, codebases, and references used to implement `github.com/Defacto2/archive/lbr`.

---

## 1. Specifications

### LUDEF5: Library File (LBR) Format Definition (Version 5)

- **Author:** Gary P. Novosielski (August 1984).
- **Online Reference:** [https://www.seasip.info/Cpm/ludef5.html](https://www.seasip.info/Cpm/ludef5.html)
- **Referenced For:**
  - 128-byte fixed sector layout and 32-byte directory entry structure.
  - Entry status codes (`0x00` active, `0xFE` deleted, `0xFF` unused).
  - Pad byte count calculation (`rawSize - padCount`) and `0x1A` sector padding.
  - CP/M 3 date encoding (days since December 31, 1977) and packed timestamps.
  - CCITT/XMODEM CRC-16 (`0x1021` polynomial) over member data.

---

## 2. Codebases & Utilities

### lar (Library Archiver)

- **Author:** Stephen C. Hemminger (1986).
- **Status:** Public Domain.
- **Referenced For:** UNIX extraction, directory parsing, and LBR archive handling conventions.

### The Unarchiver (`XADLBRParser`)

- **License:** GNU Lesser General Public License (LGPL).
- **Referenced For:** Verification of CRC-16 CCITT calculation over unpadded member bytes.
