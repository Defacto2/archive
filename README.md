# Defacto2 / archive

[![Go Reference](https://pkg.go.dev/badge/github.com/Defacto2/archive.svg)](https://pkg.go.dev/github.com/Defacto2/archive)
[![Go Report Card](https://goreportcard.com/badge/github.com/Defacto2/server)](https://goreportcard.com/report/github.com/Defacto2/archive)

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
    // Extract all files from an archive.
    if err := archive.Extract("path/to/archive.zip", "path/to/extract"); err != nil {
        fmt.Println(err)
    }

    // Extract a specific files from an archive.
    x := archive.Extractor{
        Source: "path/to/archive.zip",
        Destination: "path/to/extract",
    }
    if err := x.Extract("file1.txt", "file2.txt"); err != nil {
        fmt.Println(err)
    }

    // Extract all files to a temporary directory.
    path, err := archive.ExtractSource("path/to/archive.zip", "tempsubdir")
    if err != nil {
        fmt.Println(err)
    }
    fmt.Println("Extracted to:", path)

    // List the contents of an archive.
    files, err := archive.List("path/to/archive.zip", "archive.zip")
    if err != nil {
        fmt.Println(err)
    }
    for _, f := range files {
        fmt.Println(f)
    }

    // Search for a possible readme file within the list of files.
    name := archive.Readme("archive.zip", cont.Files)
    fmt.Println(name)

    // Compress a file into a new archive.
    if _, err := rezip.Compress("file1.txt", "path/to/new.zip"); err != nil {
        fmt.Println(err)
    }
    // Compress a directory into a new archive.
    if _, err = rezip.CompressDir("path/to/directory", "path/to/new.zip"); err != nil {
        fmt.Println(err)
    }
}
```