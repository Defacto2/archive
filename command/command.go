package command

const (
	Arc      = "arc"      // Arc is the arc decompression command.
	Arj      = "arj"      // Arj is the arj decompression command.
	Ansilove = "ansilove" // Ansilove is the ansilove text to image command.
	Convert  = "convert"  // Convert is the ImageMagick legacy convert command.
	Cwebp    = "cwebp"    // Cwebp is the Google create webp command.
	Gwebp    = "gif2webp" // Gwebp is the Google gif to webp command.
	HWZip    = "hwzip"    // Hwzip the zip decompression command for files using obsolete methods.
	Lha      = "lha"      // Lha is the lha/lzh decompression command.
	Magick   = "magick"   // Magick is the ImageMagick v7+ command.
	Optipng  = "optipng"  // Optipng is the PNG optimizer command.
	Tar      = "tar"      // Tar is the tar decompression command.
	// A note about unrar on linux, the installation cannot use the unrar-free package,
	// which is a poor substitute for the files this application needs to handle.
	// The unrar binary should return:
	// "UNRAR 6.24 freeware, Copyright (c) 1993-2023 Alexander Roshal".
	Unrar   = "unrar"   // Unrar is the rar decompression command.
	Unzip   = "unzip"   // Unzip is the zip decompression command.
	Zip7    = "7zz"     // Zip7 is the 7-Zip decompression command.
	ZipInfo = "zipinfo" // ZipInfo is the zip information command.
)
