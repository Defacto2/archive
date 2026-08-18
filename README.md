# Defacto2 / archive

[![Go Reference](https://pkg.go.dev/badge/github.com/Defacto2/archive.svg)](https://pkg.go.dev/github.com/Defacto2/archive)

The archive package provides compressed and stored archive file extraction and content listing functions. See the [reference documentation](https://pkg.go.dev/github.com/Defacto2/archive) for additional usage and examples.

## Usage

In your Go project, import the releaser library.

```sh
go get github.com/Defacto2/archive
```

Use the functions.

```go
import "github.com/Defacto2/archive"

func main() {
    src := filepath.Join("path", "to", "archive.zip")
    dst := filepath.Join(os.TempDir(), "extracted")

    // Extract all files from an archive.
    _ = archive.ExtractAll(context.TODO(), src, dst)

    // Extract targets or specific files from an archive.
    x := archive.Extractor{
        Source:       src,
        Destination:  dst,
    }
    _ = x.Extract(context.TODO(), "file1.txt", "file2.txt")

    // List the contents of an archive.
    names, _ := archive.Lists(src)
    for n, name := range names {
        fmt.Println(n, name)
    }

    // Search for a possible readme file within the list of files.
    filename := filepath.Base(src)
    readme := archive.Readme(filename, files...)
    fmt.Println(readme)

    // Compress a file into a new archive.
    srcFile := filepath.Join("path", "to", "textfile.txt")
    dst = filepath.Join(os.TempDir(), "testfile.zip")
    _, _ = rezip.Compress(srcFile, dst)

    // Compress a directory into a new archive.
    srcDir := filepath.Join("path", "to", "compress")
    dst = filepath.Join(os.TempDir(), "testdata.zip")
    _, _ = rezip.CompressDir(srcDir, dst)
}
```
