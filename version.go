package archive

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/Defacto2/archive/command"
)

type ProgInfo struct {
	Prog    string      // Prog is the basename of the program
	Abs     string      // Abs is the absolute path of the program
	Regular bool        // Regular is false when the program is a symlink
	Perm    os.FileMode // Perm bits of the program or symlink
	Size    int64       // Size of the program or symlink
	Output  string      // Output string when running the program
}

// Lookup the named program version and using an optional argument.
// The output will be parsed and stored as a string to [ProgInfo.Output].
func (pi *ProgInfo) Lookup(ctx context.Context, name, arg string) error {
	const format = "proginfo lookup: %s: %w"
	prog := command.Bottle(name)

	info, err := os.Lstat(prog)
	if err != nil {
		path, err := exec.LookPath(prog)
		if err != nil {
			return fmt.Errorf(format, name, err)
		}
		info, err = os.Lstat(path)
		if err != nil {
			return fmt.Errorf(format, name, err)
		}
	}

	// INFO: some programs return different outputs when you include a blank argument vs none
	var cmd *exec.Cmd
	if arg == "" {
		cmd = exec.CommandContext(ctx, prog)
	} else {
		cmd = exec.CommandContext(ctx, prog, arg)
	}
	b, err := cmd.CombinedOutput()
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		return fmt.Errorf(format, name, err)
	}

	abs, err := exec.LookPath(prog)
	if err != nil {
		abs = err.Error()
	}

	*pi = ProgInfo{
		Prog:    filepath.Base(prog),
		Abs:     abs,
		Regular: info.Mode().IsRegular(),
		Perm:    info.Mode().Perm(),
		Size:    info.Size(),
		Output:  progOutput(name, b),
	}
	return nil
}

func progOutput(name string, b []byte) string {
	firsts := []string{
		command.Arc, command.Arj,
		command.Gzip, command.HWZip,
		command.Lha, command.Tar,
		command.Unrar, command.Unzip,
		command.Zip7, command.ZipInfo,
	}

	switch {
	case slices.Contains(firsts, name):
		scanner := bufio.NewScanner(bytes.NewReader(b))
		firstLine := []byte{}
		for scanner.Scan() {
			b := bytes.TrimSpace(scanner.Bytes())
			if len(b) > 0 {
				firstLine = b
				break
			}
		}
		return string(firstLine)
	case name == command.BSDTar:
		scanner := bufio.NewScanner(bytes.NewReader(b))
		lastLine := []byte{}
		for scanner.Scan() {
			b := bytes.TrimSpace(scanner.Bytes())
			if len(b) > 0 {
				lastLine = b
			}
		}
		return string(lastLine)
	default:
		return string(b)
	}
}

const n = 14 // n is the number of array objects used for ProgInfos

// ProgInfos returns the program versions of the various commands
// referenced by this archive package.
func ProgInfos(ctx context.Context) ([n]ProgInfo, error) {
	vers := [n]ProgInfo{}
	cmds := [n]string{
		command.Zip7,
		command.Arc,
		command.Arj,
		command.BSDTar,
		command.Cab,
		command.Gzip,
		command.HWZip,
		command.Lha,
		command.Tar,
		command.Unrar,
		command.Unzip,
		command.ZipInfo,
		command.Lsar,
		command.Unar,
	}
	const v = "--version"
	flgs := [n]string{
		"",
		"",
		"",
		"-h",
		v,
		v,
		"",
		"",
		v,
		"",
		"-h",
		"",
		"-version",
		"-version",
	}
	for i, name := range cmds {
		var inf ProgInfo
		err := inf.Lookup(ctx, name, flgs[i])
		if err != nil {
			return vers, err
		}
		vers[i] = inf
	}
	return vers, nil
}
