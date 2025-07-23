package pkzip_test

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Defacto2/archive/command"
	"github.com/Defacto2/archive/pkzip"
	"github.com/nalgeon/be"
)

func td(name string) string {
	_, file, _, usable := runtime.Caller(0)
	if !usable {
		panic("runtime.Caller failed")
	}
	d := filepath.Join(filepath.Dir(file), "..")
	return filepath.Join(d, "testdata", name)
}

func TestPkzip(t *testing.T) {
	t.Parallel()
	comps, err := pkzip.Methods(td("PKZ204EX.TXT"))
	be.Err(t, err)
	be.Equal(t, comps, nil)
	comps, err = pkzip.Methods(td("PKZ204EX.ZIP"))
	be.Err(t, err, nil)
	be.Equal(t, pkzip.Deflated, comps[1])
	be.Equal(t, pkzip.Stored, comps[0])
	comps, err = pkzip.Methods(td("PKZ80A1.ZIP"))
	be.Err(t, err, nil)
	be.Equal(t, pkzip.Shrunk, comps[1])
	be.Equal(t, pkzip.Stored, comps[0])
	comps, err = pkzip.Methods(td("PKZ80A1.ZIP"))
	be.Err(t, err, nil)
	be.Equal(t, pkzip.Shrunk.String(), comps[1].String())
	be.Equal(t, pkzip.Stored.String(), comps[0].String())
	comps, err = pkzip.Methods(td("PKZ110EI.ZIP"))
	be.Err(t, err, nil)
	be.Equal(t, "[Stored Imploded]", fmt.Sprint(comps))
	be.True(t, !comps[1].Zip())
	usable, err := pkzip.Zip(td("PKZ204EX.TXT"))
	be.Err(t, err)
	be.True(t, !usable)
	usable, err = pkzip.Zip(td("PKZ204EX.ZIP"))
	be.Err(t, err, nil)
	be.True(t, usable)
	usable, err = pkzip.Zip(td("PKZ80A1.ZIP"))
	be.Err(t, err, nil)
	be.True(t, !usable)
	const invalid = 999
	comp := pkzip.Compression(invalid)
	be.Equal(t, "Reserved", comp.String())
}

func TestExitStatus(t *testing.T) {
	t.Parallel()
	app, err := exec.LookPath(command.Unzip)
	be.Err(t, err, nil)
	err = exec.CommandContext(context.Background(), app, "-T", "archive.zip").Run()
	be.Err(t, err)
	diag := pkzip.ExitStatus(err)
	be.Equal(t, pkzip.ZipNotFound, diag)
	be.Equal(t, "Zip file not found", diag.String())
}
