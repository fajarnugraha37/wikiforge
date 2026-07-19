package pathutil

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExternalToolPathCrossPlatform(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"windows drive", `C:\Users\nugra\My Project\資料\prompt.md`, `C:/Users/nugra/My Project/資料/prompt.md`},
		{"windows extended", `\\?\C:\very long\project\prompt.md`, `C:/very long/project/prompt.md`},
		{"unc", `\\server\share\My Project\prompt.md`, `//server/share/My Project/prompt.md`},
		{"extended unc", `\\?\UNC\server\share\folder\prompt.md`, `//server/share/folder/prompt.md`},
		{"mac", `/Users/fajar/My Project/prompt.md`, `/Users/fajar/My Project/prompt.md`},
		{"linux unicode", `/home/fajar/資料/prompt.md`, `/home/fajar/資料/prompt.md`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExternalToolPath(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
			if strings.ContainsAny(got, `\"`) {
				t.Fatalf("external path contains unsafe quoting or backslashes: %q", got)
			}
		})
	}
}

func TestExternalToolPathRejectsUnsafeAndRelative(t *testing.T) {
	for _, value := range []string{`relative/path.md`, `"C:\prompt.md"`, "/tmp/a\npath", ""} {
		if _, err := ExternalToolPath(value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}

func TestNormalizeRelativeAcceptsBothSeparators(t *testing.T) {
	got, err := NormalizeRelative(`apps\catalog/api`)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("apps", "catalog", "api")
	if got != want {
		t.Fatalf("got %q want %q on %s", got, want, runtime.GOOS)
	}
	for _, value := range []string{`../escape`, `/absolute`, `C:\absolute`, `\\server\share`} {
		if _, err := NormalizeRelative(value); err == nil {
			t.Fatalf("expected rejection for %q", value)
		}
	}
}

func TestResolveKeepsRelativePathsUnderBase(t *testing.T) {
	base := t.TempDir()
	want := filepath.Join(base, "config", "wikiforge.yaml")
	got, err := Resolve(base, `config\wikiforge.yaml`)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveAcceptsNativeAbsolutePath(t *testing.T) {
	base := t.TempDir()
	absolute := filepath.Join(base, "config", "wikiforge.yaml")
	got, err := Resolve(base, absolute)
	if err != nil {
		t.Fatal(err)
	}
	if got != absolute {
		t.Fatalf("got %q want %q", got, absolute)
	}
}

func TestResolveRejectsForeignWindowsAbsolutePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows drive paths are native on Windows")
	}
	if _, err := Resolve(t.TempDir(), `C:\workspace\wikiforge.yaml`); err == nil {
		t.Fatal("expected a foreign Windows absolute path to be rejected")
	}
}

func TestValidatePortableSegment(t *testing.T) {
	for _, value := range []string{"sentinel", "order-service.v2", "資料"} {
		if err := ValidatePortableSegment(value); err != nil {
			t.Fatalf("expected %q to be portable: %v", value, err)
		}
	}
	for _, value := range []string{"../escape", `a\\b`, "a/b", "CON", "nul.txt", "trailing.", "trailing ", "a:b", ""} {
		if err := ValidatePortableSegment(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
