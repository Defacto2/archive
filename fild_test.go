package archive_test

import (
	"fmt"
	"testing"

	"github.com/Defacto2/archive"
)

func ExampleReadme() {
	name := archive.Readme("APP.ZIP", "APP.EXE", "APP.TXT",
		"APP.BIN", "APP.DAT", "STUFF.DAT")
	fmt.Println(name)
	// Output: APP.TXT
}

func TestReadme(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		files    []string
		want     string
	}{
		{"NFO #1", "APP.ZIP", []string{"APP.EXE", "APP.NFO"}, "APP.NFO"},
		{"TXT #1", "APP.ZIP", []string{"APP.EXE", "APP.TXT"}, "APP.TXT"},
		{"NFO #2", "APP.ZIP", []string{"APP.EXE", "STUFF.NFO"}, "STUFF.NFO"},
		{"DIZ #1", "APP.ZIP", []string{"APP.EXE", "FILE_ID.DIZ", "APP.DIZ"}, "FILE_ID.DIZ"},
		{"DIZ #2", "APP.ZIP", []string{"APP.EXE", "APP.DIZ"}, "APP.DIZ"},
		{"TXT #2", "APP.ZIP", []string{"APP.EXE", "STUFF.TXT"}, "STUFF.TXT"},
		{"DIZ #3", "APP.ZIP", []string{"APP.EXE", "STUFF.DIZ"}, "STUFF.DIZ"},
		{"None", "APP.ZIP", []string{"APP.EXE", "STUFF.DAT"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := archive.Readme(tt.filename, tt.files...)
			if got != tt.want {
				t.Errorf("Readme() = %v, want %v", got, tt.want)
			}
		})
	}
}
