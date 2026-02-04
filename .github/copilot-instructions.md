# Copilot Instructions for Archive Package

## Overview

The `archive` package is a Go library for extracting and listing contents of various compressed and stored archive file formats (ZIP, RAR, TAR, 7-Zip, ARC, ARJ, CAB, LHA, and their variants). The package delegates to external system commands for legacy format support while handling modern formats with Go libraries.

## Build, Test, and Lint Commands

This project uses **Task** as its task runner (https://taskfile.dev/). Run `task --list-all` to see all available tasks.

**Common Commands:**
```bash
# Run all tests
task test

# Run tests with race detection (slower)
task testr

# Lint and format code
task lint

# Update/patch dependencies
task update  # Full update
task patch   # Patch-level updates only

# Static nil analysis
task nil

# View documentation locally
task doc  # Starts pkgsite on localhost:8090
```

**Direct Go commands:**
```bash
# Run a single test file
go test -count 1 ./... -run TestName

# Run tests in a specific package
go test -count 1 ./command

# Run with verbose output
go test -v -count 1 ./...
```

## Architecture & Key Concepts

### File Format Support Strategy

The package uses a **dual approach**:
- **Modern formats** (ZIP, 7-Zip): Handled via Go libraries (internal `pkzip` package for ZIP variants)
- **Legacy formats** (ARC, ARJ, LHA, RAR, etc.): Delegated to external system commands (defined in `command/command.go`)

### Main Entry Points

1. **Content Reading** (`Content` type in `archive.go`)
   - `Content.Read(src string)` - Detects format and populates `Files` slice
   - Uses magic number detection (via `magicnumber` package) to identify format
   - Dispatches to format-specific readers (e.g., `ARC()`, `ZIP()`, `TAR()`)

2. **Extraction** (`Extractor` type in `archive.go`)
   - `Extractor.Extract(targets ...string)` - Extracts specific files
   - `ExtractAll(src, dst string)` - Convenience function for full extraction
   - Format-specific extraction methods: `ZIP()` and `Generic()` (for external commands)

3. **Utility Functions**
   - `List(src, filename string)` - Lists archive contents without extraction
   - `MagicExt(src string)` - Determines file extension from magic bytes
   - `HardLink(require, src string)` - Validates required external programs exist

### Package Structure

- **`archive.go`** - Core types (`Content`, `Extractor`, `Run`) and dispatching logic
- **`archive_test.go`** - Integration tests for read/extract operations
- **Format-specific files** (`arc.go`, `zip.go`, `tar.go`, etc.) - Each has:
  - Public receiver method on `Content` (e.g., `func (c *Content) ARC(src string)`)
  - Private parsing functions for command output
- **`command/`** - External command name constants and timeout values
- **`pkzip/`** - Custom ZIP reader for obsolete compression methods (deflate, implode, shrink, store)
- **`rezip/`** - ZIP creation utilities
- **`find.go`** - File searching within archives
- **`fild_test.go`** - Tests for find functionality

### Error Handling Pattern

The package defines custom errors as module-level vars (not wrapped in enums):
```go
var (
    ErrDest           = errors.New("destination is empty")
    ErrNotArchive     = errors.New("file is not an archive")
    ErrNotImplemented = errors.New("archive format is not implemented")
    // ... more errors
)
```

**Wrapping convention:** Use `fmt.Errorf("function_name %w", err)` for propagating errors with context.

### Format Detection

1. **Magic byte detection** - Uses `magicnumber.Signature` from `Defacto2/magicnumber` package
2. **Extension fallback** - Attempts format based on file extension if magic detection fails
3. **Special handling:**
   - LHA format detection via magic bytes vs file extension (some files mislabeled as `.zip`)
   - TAR.GZ detection requires both magic bytes and file existence check (no distinct magic for gzip+tar combination)

### External Command Dependencies

The package requires these system programs depending on format support:
- `arc`, `arj`, `lha`, `unrar`, `7zz`, `gcab`, `tar`, `gzip` - See `command/command.go` for list

**Timeout values:**
- `TimeoutList` (2 seconds) - For listing archive contents
- `TimeoutExtract` (15 seconds) - For extraction operations
- `TimeoutDefunct` (5 seconds) - For legacy format handling

## Key Conventions

### Naming

- **Extension constants** use format: `<format>x` (e.g., `zipx`, `rarx`, `arcx` for `.zip`, `.rar`, `.arc`)
- **Private format functions** prefixed with format name in lowercase (e.g., `arcFiles()`, `zipInto()`)
- **Receiver methods** are public and named after format (e.g., `ARC()`, `ZIP()`, `TAR()`)

### Testing

- **Test files** paired with source files (e.g., `archive_test.go`, `pkzip_test.go`)
- **Test flag** use `-count 1` to disable caching (standard in this project)
- **Testdata** directory contains sample archive files for testing

### Code Style

- Linted with **golangci-lint** using `.golangci.yaml` configuration
- Formatted with **gofumpt** (stricter than gofmt)
- Uses **goimports** and **gci** for import organization
- Nil dereference analysis via **nilaway** tool
- US English locale for spell checking

### Import Organization

Imports are organized by golangci-lint formatters (gci):
1. Standard library
2. Third-party packages
3. Local package imports

### Documentation

- Package-level docs in `archive.go` include:
  - High-level purpose
  - List of supported formats
  - References to required external programs with links
  - Usage examples showing all main API functions
- Function docs follow standard Go conventions (start with function name)

## Related Dependencies

- `github.com/Defacto2/helper` - Helper utilities
- `github.com/Defacto2/magicnumber` - Magic byte detection for file formats
- `github.com/nalgeon/be` - Error assertion helper
- External linters: `nilaway`, `golangci-lint`, `gofumpt`
