package sanitize

import (
	"path/filepath"
	"strings"
)

// Name applies [filepath.Localize] on the path and
// replaces any occurrences of following:
//   - "/" with "_"
//   - "\x00" (null) with ""
func Name(path string) string {
	s, err := filepath.Localize(path)
	if err == nil {
		return s
	}

	r := strings.NewReplacer(
		"/", "_", "\x00", "",
	)
	clean := strings.TrimSpace(r.Replace(s))
	if clean == "" {
		return "unnamed_file"
	}

	return clean
}

// WindowsName applies [filepath.Localize] on the base name.
// It also replaces any reserved characters with underscores,
// and all ASCII control characters are removed.
//
// See: https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file
func WindowsName(path string) string {
	s := path
	if local, err := filepath.Localize(path); err == nil && local != "" {
		s = local
	}

	clean := strings.TrimSpace(windowsNaming.Replace(s))
	if clean == "" {
		return "unnamed_file"
	}

	return clean
}

var windowsNaming = windowsNamingReplacer() //nolint:gochecknoglobals

func windowsNamingReplacer() *strings.Replacer {
	const asciiControls = 31
	const size = (9 + asciiControls + 1) * 2
	oldnew := make([]string, 0, size)

	const remove, sep = "", "_"
	oldnew = append(oldnew,
		"<", sep,
		">", sep,
		":", sep,
		`"`, sep,
		"/", sep,
		"\\", sep,
		"|", sep,
		"?", sep,
		"*", sep,
	)
	for b := byte(0); b <= asciiControls; b++ {
		oldnew = append(oldnew, string(b), remove)
	}

	return strings.NewReplacer(oldnew...)
}
