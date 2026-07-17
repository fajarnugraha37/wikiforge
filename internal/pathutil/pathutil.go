package pathutil

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
)

var windowsDriveAbs = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// IsAbsoluteAny reports whether p is an absolute POSIX, Windows drive, UNC,
// or Windows extended-length path. It is intentionally independent of the
// host OS so shared configuration can be validated consistently.
func IsAbsoluteAny(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return false
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") || windowsDriveAbs.MatchString(p) {
		return true
	}
	return strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, `//`) ||
		strings.HasPrefix(p, `\\?\\`) || strings.HasPrefix(p, `//?/`)
}

// NormalizeRelative accepts either slash style and returns a native relative
// path. Absolute paths, parent escapes, control characters, and NUL are rejected.
func NormalizeRelative(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "." {
		return "", nil
	}
	if hasUnsafeCharacters(value) {
		return "", fmt.Errorf("path contains a double quote, control character, or NUL")
	}
	if IsAbsoluteAny(value) {
		return "", fmt.Errorf("path must be relative")
	}
	value = strings.ReplaceAll(value, `\`, `/`)
	cleanSlash := pathpkg.Clean(value)
	if cleanSlash == ".." || strings.HasPrefix(cleanSlash, "../") {
		return "", fmt.Errorf("path must not escape its base directory")
	}
	if cleanSlash == "." {
		return "", nil
	}
	return filepath.FromSlash(cleanSlash), nil
}

// Resolve expands a leading home marker, accepts either separator style, and
// returns a cleaned absolute native path relative to base.
func Resolve(base, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if hasUnsafeCharacters(value) {
		return "", fmt.Errorf("path contains a double quote, control character, or NUL")
	}
	expanded, err := expandHome(value)
	if err != nil {
		return "", err
	}
	expanded = strings.ReplaceAll(expanded, `\`, `/`)
	native := filepath.FromSlash(expanded)
	if !filepath.IsAbs(native) {
		native = filepath.Join(base, native)
	}
	return filepath.Abs(filepath.Clean(native))
}

// ExternalToolPath converts an already absolute native path into an absolute,
// quote-free path that Node-based tools accept on Windows, macOS, and Linux.
// Windows paths use forward slashes to avoid backslash escaping by models or
// JavaScript tooling. Extended-length prefixes are removed before transport.
func ExternalToolPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("path is empty")
	}
	if hasUnsafeCharacters(value) {
		return "", fmt.Errorf("path contains a double quote, control character, or NUL")
	}

	p := strings.ReplaceAll(value, `\`, `/`)
	lower := strings.ToLower(p)
	switch {
	case strings.HasPrefix(lower, "//?/unc/"):
		p = "//" + p[len("//?/UNC/"):]
	case strings.HasPrefix(lower, "//?/"):
		p = p[len("//?/"):]
	}

	switch {
	case windowsDriveAbs.MatchString(p):
		drive := p[:2]
		rest := pathpkg.Clean("/" + strings.TrimLeft(p[2:], "/"))
		if rest == "/." {
			rest = "/"
		}
		return drive + rest, nil
	case strings.HasPrefix(p, "//"):
		rest := strings.TrimLeft(p, "/")
		if rest == "" {
			return "", fmt.Errorf("UNC path is missing server and share")
		}
		clean := pathpkg.Clean("/" + rest)
		parts := strings.Split(strings.TrimPrefix(clean, "/"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return "", fmt.Errorf("UNC path is missing server or share")
		}
		return "//" + strings.TrimPrefix(clean, "/"), nil
	case strings.HasPrefix(p, "/"):
		return pathpkg.Clean(p), nil
	default:
		return "", fmt.Errorf("path is not absolute: %s", value)
	}
}

// ValidatePortableSegment ensures an identifier can safely be used as one
// directory or file-name segment on Windows, macOS, and Linux.
func ValidatePortableSegment(value string) error {
	raw := value
	value = strings.TrimSpace(value)
	if raw != value {
		return fmt.Errorf("segment must not begin or end with whitespace")
	}
	if value == "" {
		return fmt.Errorf("segment is empty")
	}
	if value == "." || value == ".." {
		return fmt.Errorf("segment must not be %q", value)
	}
	if strings.ContainsAny(value, `<>:"/\|?*`) || strings.ContainsAny(value, "\x00\r\n\t") {
		return fmt.Errorf("segment contains characters unsafe on Windows, macOS, or Linux")
	}
	if strings.HasSuffix(value, ".") || strings.HasSuffix(value, " ") {
		return fmt.Errorf("segment must not end with a dot or space")
	}
	base := strings.ToUpper(strings.TrimSuffix(value, filepath.Ext(value)))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return fmt.Errorf("segment uses a Windows reserved device name")
	}
	return nil
}

func expandHome(value string) (string, error) {
	normalized := strings.ReplaceAll(value, `\`, `/`)
	if normalized != "~" && !strings.HasPrefix(normalized, "~/") {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if normalized == "~" {
		return home, nil
	}
	return filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(normalized, "~/"))), nil
}

func hasUnsafeCharacters(value string) bool {
	return strings.ContainsAny(value, "\x00\r\n\"")
}
