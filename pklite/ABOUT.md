# About the pklite package

The `pklite` package decompresses MS-DOS programs that were packed with PKWARE's PKLITE.
It supports `.COM` files (v1.00 beta to v2.01) and unscrambled `.EXE` files (v1.05 to v1.15).

## References and sources

### Primary source

- **Deark**, `modules/pklite.c`, by Jason Summers, MIT licensed.
  <https://github.com/jsummers/deark/blob/master/modules/pklite.c>

  This is the only implementation that was read. The package is derived from the parts of it listed below, and it was fetched on 2026-11-20.

  | Taken from Deark                                                                                                      | Used in                                                                 |
  | --------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
  | The 16-bit little-endian, LSB-first bit reader, with its eager refill                                                 | `bitReader` in `pklite.go`                                              |
  | The static Huffman code tables for match lengths (small and large) and offsets                                        | `smallLengthCodes`, `largeLengthCodes` and `offsetCodes` in `pklite.go` |
  | The LZ77 token layout, special codes (`0xff` stop, `0xfe` separator, `0xfd` uncompressed area) and length bias values | `decode` in `pklite.go`                                                 |
  | The "extra" compression, where literals are XORed with the bit count                                                  | `decode` in `pklite.go`                                                 |
  | The COM stub layout: intro, copier, then the decompressor that holds the data address                                 | `findDataPos` in `pklite.go`                                            |
  | The EXE stub layout and the byte patterns that locate the copier, the decompressor and the compressed data            | `analyze` in `exe.go`                                                   |
  | The patterns that tell large from small and normal from extra compression                                             | `analyze` in `exe.go`                                                   |
  | The short and long compressed relocation tables, and the register footer                                              | `readRelocs` and `readLongRelocs` in `exe.go`                           |
  | Reading the stored original EXE header, or rebuilding one with a minimum-memory estimate                              | `originalHeader` and `reconstructHeader` in `exe.go`                    |

  Deark's own header acknowledges Sergei Kolzun (private communication) for information about the v1.20 formats. That work is not used here.

### Related projects named in the code or in Deark

- **oldunzip**, by Jason Summers. This is the other sibling-package reference cited in `unshrink`. It was not read for pklite.
  <https://github.com/jsummers/oldunzip>
- **pkla**, by Jason Summers. Deark's comments point to it as a better tool for reporting PKLITE file details. It was not consulted.

### Test data

The tests use the files in the `testdata` directory of the `pklite` package.
They are not copyrighted third-party programs.

| File           | Description                                                                                                                                                        |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `D2.COM`       | the uncompressed original                                                                                                                                          |
| `D2_V1B.COM`   | PKLITE v1.00 beta COM, compressed from `D2.COM`                                                                                                                    |
| `D2_V115.COM`  | PKLITE v1.15 COM, compressed from `D2.COM`                                                                                                                         |
| `LOR.EXE`      | the uncompressed original EXE                                                                                                                                      |
| `LOR_V115.EXE` | PKLITE v1.15 EXE, compressed from `LOR.EXE`                                                                                                                        |
| `LOR_V1E.EXE`  | PKLITE EXE with the extra compression, compressed from `LOR.EXE`. Identical to `LOR_V3.EXE` except for one byte of the copyright line (1990-91 instead of 1990-95) |
| `LOR_V3A.EXE`  | the same PKLITE as `LOR_V3.EXE` with the `-a` flag, compressed from `LOR.EXE`                                                                                      |
| `LOR_V3.EXE`   | PKLITE EXE, compressed from `LOR.EXE` with a later PKLITE (1990-95 copyright line). The stub reports v1.12 with the extra compression flag                         |

The COM files and `LOR_V115.EXE` decompress to a result byte-identical to their originals, which is a true reference.
This includes the EXE, so the relocation table, the stored original header and the code are all verified.

`LOR_V3.EXE` uses the extra compression, which does not store the original header, so it is rebuilt.
Its program code and starting registers match the original, and its relocation table refers to the same addresses (as segment and offset pairs it is encoded differently), but the header differs (16-byte instead of 512-byte alignment, and an estimated minimum memory of 0x140 paragraphs against the original 0).

`LOR_V1E.EXE` has the same compressed data and rebuilt header as `LOR_V3.EXE`, so it gives the same result.

`LOR_V3A.EXE` was made from `LOR.EXE` with the same PKLITE and its `-a` flag, and it decompresses byte-identical to the original.
Compared with `LOR_V3.EXE`, it uses the normal compression with a stored copy of the original header instead of the extra compression.
`LOR_V3A.EXE` is therefore a true reference for the same program.
It is not known why the flag changes the compression type, and the files do not have an overlay.

Earlier development also used commercial and scene programs of v1.05 and v1.12, with the large and extra compression.
They were removed because they are copyrighted.
The large compression and the extra compression are covered by the EXE tests, but the v1.05 and v1.12 stubs are not.

## Repository and library context

- `github.com/Defacto2/magicnumber`, v1.4.9, `executable.go` `Pklite()`: detects a PKLITE EXE by the string `PKLITE` at offset 30. It does not detect COM files, which is why `detect()` in the `archive` package also calls `pklite.IsCOM`.
- The sibling packages `unshrink`, `explode` and `expand` in this repository were used as the style template, for the package layout, example tests and error naming.

## Credits

Thanks to everyone whose work this package stands on:

- **Jason Summers** - author of Deark, the source this package is ported from. He also wrote pkla and oldunzip, which Deark and the sibling packages point to as references.
- **Sergei Kolzun** - credited by Deark for private communication about the v1.20 formats. That information is not used in this package, but it is part of what makes Deark's PKLITE research possible.
- **Andrew Milner / Continental Software** - Deark's comments cite the customised "v1.23" PKLITE files in RemoteAccess v1.11. The package doesn't support that variant yet.
- **Phil Katz and PKWARE, Inc.** - the creators of PKLITE (1990 to 1991). The stubs in the test files still carry the line `PKLITE Copr. 1990-91 PKWARE Inc.`
- **The authors of the test programs** - the original creators of D2.COM and LOR.EXE, and of the programs used in earlier development.
- **Ben and the Defacto2 team** - for the `archive` and `magicnumber` packages, the sibling legacy-method packages, and for supplying and identifying the test files.
- **`github.com/nalgeon/be`** - the test assertion library used by the tests (the module's author is credited on its GitHub page).
- **AI assistance** - this package was written with Antigravity (Google DeepMind), an AI coding agent, working from the Deark source above. All of its output should be reviewed as such.

## Not used

No documentation, specification or source other than Deark was consulted. In particular, PKWARE's own PKLITE documentation and `unp`, `UNPKLITE` and DOSBox were not used.

## Known gaps

- v1.14 and later EXEs with a scrambled decompressor (including 1.50 and 2.01).
- v1.20 files.
- The uncompressed-area marker (`0xfd`) used by PKZIP.EXE.
- Deark's custom v1.23 literal variant (XOR `0xff`).
- The v1.00 beta EXE layouts. The beta COM layout is supported.
- The load-high option.
