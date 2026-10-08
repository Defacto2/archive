// Package command lists the known archiving and decompression application names.
package command

import (
	"os"
	"path/filepath"
	"time"
)

// A note about unrar: On Linux there are incompatible variants of unrar.
// This package cannot use the common unrar-free application. It unfortunately, is
// incomplete and is incompatible with many .rar files this package needs to handle.
//
// When used on Linux, the unrar application should provide the following copyright:
// "UNRAR 6.24 freeware, Copyright (c) 1993-2023 Alexander Roshal".

const (
	Arc    = "arc"    // Arc is the arc decompression command.
	Arj    = "arj"    // Arj is the arj decompression command.
	BSDTar = "bsdtar" // BSDTar is the tar decompression command.
	Cab    = "gcab"   // Cab is the gcab decompression command for Microsoft Cabinet.
	Lha    = "lha"    // Lha is the lha/lzh decompression command.
	Lsar   = "lsar"   // Lsar is The Unarchive list command usable on multiple types.
	Unar   = "unar"   // Unar is The Unarchiver decompression command usable on multiple types.
	Unrar  = "unrar"  // Unrar is the rar decompression command.
	Unzip  = "unzip"  // Unzip is the zip decompression command.
	Zip7   = "7zz"    // Zip7 is the 7-Zip decompression command.
)

const (
	// HomeBrew is the default prefix path for Linux command overwrites.
	HomeBrew = "/home/linuxbrew/.linuxbrew/bin"
)

const (
	TimeoutList    = 2 * time.Second  // TimeoutList value in seconds for the command running in a background context.
	TimeoutDefunct = 5 * time.Second  // TimeoutDefunct is the maximum time allowed for the defunct file extraction.
	TimeoutExtract = 15 * time.Second // TimeoutExtract is the maximum time allowed for the archive extraction.
)

// Bottle is a workaround for the use of HomeBrew formula that are newer
// than the programs or tools provided by the host operating system.
// It checks the path and mode of the named command, and returns an
// absolute path to the HomeBrew install for use with the Go exec package.
// If there is no valid bottle, it returns the name string.
//
// For example, in 2026, Ubuntu 24 LTS uses old copies of lsar/unar
// that cannot be upgraded, but have a extraction bug while dealing
// with LHA/LZH archives. Bottle can provide a path to newer, fixed versions.
func Bottle(name string) string {
	prog := filepath.Join(HomeBrew, name)
	info, err := os.Stat(prog)
	if err != nil {
		return name
	}
	mode := info.Mode()
	if !mode.IsRegular() {
		return name
	}
	const exeBit = 0o111
	if ok := mode&exeBit != 0; !ok {
		return name
	}
	return prog
}
