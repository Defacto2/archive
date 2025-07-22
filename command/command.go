// Package command lists the known archiving and decompression application names.
package command

// A note about unrar: On Linux there are incompatible variants of unrar.
// This package cannot use the common unrar-free application. It unfortunately, is
// incomplete and is incompatible with many .rar files this package needs to handle.
//
// When used on Linux, the unrar application should provide the following copyright:
// "UNRAR 6.24 freeware, Copyright (c) 1993-2023 Alexander Roshal".

const (
	Arc     = "arc"     // Arc is the arc decompression command.
	Arj     = "arj"     // Arj is the arj decompression command.
	BSDTar  = "bsdtar"  // BSDTar is the tar decompression command.
	Cab     = "gcab"    // Cab is the gcab decompression command for Microsoft Cabinet.
	Gzip    = "gzip"    // Gzip is the gzip decompression command.
	HWZip   = "hwzip"   // Hwzip the zip decompression command for files using obsolete methods.
	Lha     = "lha"     // Lha is the lha/lzh decompression command.
	Tar     = "tar"     // Tar is the tar decompression command.
	Unrar   = "unrar"   // Unrar is the rar decompression command.
	Unzip   = "unzip"   // Unzip is the zip decompression command.
	Zip7    = "7zz"     // Zip7 is the 7-Zip decompression command.
	ZipInfo = "zipinfo" // ZipInfo is the zip information command.
)
