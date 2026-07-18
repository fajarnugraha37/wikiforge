package evidence

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func ChangedPaths(root, previousRevision, revision string) ([]string, bool, string, error) {
	if revision == "" {
		revision = gitRevision(root)
	}
	if previousRevision == "" {
		return trackedWorkingTreePaths(root), true, "previous revision is unavailable; full-scope scan", nil
	}
	if revision != "" && previousRevision != revision {
		if err := git(root, "merge-base", "--is-ancestor", previousRevision, revision); err != nil {
			return trackedWorkingTreePaths(root), true, "Git history was rewritten or revisions are unrelated; full-scope scan", nil
		}
	}
	set := map[string]bool{}
	if previousRevision != revision {
		for _, path := range gitNames(root, "diff", "--name-only", previousRevision, revision) {
			set[path] = true
		}
	}
	for _, path := range gitNames(root, "diff", "--name-only", revision) {
		set[path] = true
	}
	for _, path := range gitNames(root, "diff", "--cached", "--name-only") {
		set[path] = true
	}
	for _, path := range gitNames(root, "ls-files", "--others", "--exclude-standard") {
		set[path] = true
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		if path != "" {
			paths = append(paths, filepath.ToSlash(filepath.Clean(path)))
		}
	}
	sort.Strings(paths)
	return paths, false, "Git diff from the last successful revision", nil
}

func gitNames(root string, args ...string) []string {
	b, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, filepath.ToSlash(line))
		}
	}
	return out
}

func trackedWorkingTreePaths(root string) []string {
	paths := gitNames(root, "ls-files", "-co", "--exclude-standard")
	if len(paths) == 0 {
		return nil
	}
	return paths
}

func git(root string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
