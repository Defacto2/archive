//nolint:gochecknoglobals
package sanitize

import (
	"path/filepath"
	"strings"
)

var sanitizer = strings.NewReplacer(
	"/", "_", "\x00", "",
)

// Name applies [filepath.Localize] on the path and
// replaces any occurrences of following:
//   - "/" with "_"
//   - "\x00" (null) with ""
func Name(path string) string {
	// filepath localize does not permit root paths
	p := strings.TrimLeft(path, "/")
	s, err := filepath.Localize(p)
	if err == nil {
		return s
	}

	clean := strings.TrimSpace(sanitizer.Replace(s))
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

	local, err := filepath.Localize(path)
	if err == nil && local != "" {
		s = local
	}

	clean := strings.TrimSpace(windowsNaming.Replace(s))
	if clean == "" {
		return "unnamed_file"
	}

	return clean
}

var windowsNaming = windowsNamingReplacer()

func windowsNamingReplacer() *strings.Replacer {
	const asciiControls = 31

	const size = (9 + asciiControls + 1) * 2
	oldnew := make([]string, 0, size)

	const sep = "_"
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

	const remove = ""
	for b := byte(0); b <= asciiControls; b++ {
		oldnew = append(oldnew, string(b), remove)
	}

	return strings.NewReplacer(oldnew...)
}
