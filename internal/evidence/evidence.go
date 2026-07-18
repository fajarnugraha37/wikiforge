package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fajarnugraha37/wikiforge/internal/planner"
)

const (
	SchemaVersion          = 1
	MaxEvidenceBytes int64 = 4 * 1024 * 1024
)

type EvidenceType string

const (
	EvidenceSourceFile EvidenceType = "source-file"
	EvidenceTest       EvidenceType = "test"
	EvidenceConfig     EvidenceType = "configuration"
	EvidenceGenerated  EvidenceType = "generated"
)

type EvidenceAuthority string

const (
	AuthorityTracked   EvidenceAuthority = "tracked"
	AuthorityUntracked EvidenceAuthority = "untracked"
)

type EvidenceReference struct {
	ID           string            `json:"id"`
	RepositoryID string            `json:"repositoryId"`
	Revision     string            `json:"revision"`
	Path         string            `json:"path"`
	Symbol       string            `json:"symbol,omitempty"`
	LineStart    int               `json:"lineStart"`
	LineEnd      int               `json:"lineEnd"`
	EvidenceType EvidenceType      `json:"evidenceType"`
	ContentHash  string            `json:"contentHash"`
	Authority    EvidenceAuthority `json:"authority"`
}

type EvidenceDependency struct {
	EvidenceID string `json:"evidenceId"`
	PageID     string `json:"pageId"`
	SectionID  string `json:"sectionId"`
	EntryID    string `json:"entryId,omitempty"`
}

type SkippedEvidence struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type Index struct {
	SchemaVersion int                  `json:"schemaVersion"`
	RepositoryID  string               `json:"repositoryId"`
	Revision      string               `json:"revision"`
	References    []EvidenceReference  `json:"references"`
	Dependencies  []EvidenceDependency `json:"dependencies"`
	Skipped       []SkippedEvidence    `json:"skipped,omitempty"`
	CacheHits     int                  `json:"cacheHits,omitempty"`
	CacheMisses   int                  `json:"cacheMisses,omitempty"`
}

type cacheEntry struct {
	Path         string       `json:"path"`
	Size         int64        `json:"size"`
	ModifiedNS   int64        `json:"modifiedNs"`
	GitObject    string       `json:"gitObject,omitempty"`
	Hash         string       `json:"hash"`
	Lines        int          `json:"lines"`
	Binary       bool         `json:"binary"`
	EvidenceType EvidenceType `json:"evidenceType"`
}

type evidenceCache struct {
	Version int                   `json:"version"`
	Entries map[string]cacheEntry `json:"entries"`
}

type ChangeImpact struct {
	SchemaVersion    int                 `json:"schemaVersion"`
	RepositoryID     string              `json:"repositoryId"`
	PreviousRevision string              `json:"previousRevision,omitempty"`
	Revision         string              `json:"revision"`
	ChangedPaths     []string            `json:"changedPaths"`
	AffectedPages    map[string][]string `json:"affectedPages"`
	AffectedUnits    map[string][]string `json:"affectedUnits"`
	FullScan         bool                `json:"fullScan"`
	Reason           string              `json:"reason"`
}

type CoverageState string

const (
	NotInspected        CoverageState = "Not inspected"
	Discovered          CoverageState = "Discovered"
	PartiallyDocumented CoverageState = "Partially documented"
	Documented          CoverageState = "Documented"
	Verified            CoverageState = "Verified"
	ConflictingEvidence CoverageState = "Conflicting evidence"
	Stale               CoverageState = "Stale"
	NotApplicable       CoverageState = "Not applicable"
)

type CoverageEntry struct {
	ID          string        `json:"id"`
	Scope       string        `json:"scope"`
	ComponentID string        `json:"componentId,omitempty"`
	UnitID      string        `json:"unitId,omitempty"`
	ConcernPack string        `json:"concernPack,omitempty"`
	State       CoverageState `json:"state"`
	PageIDs     []string      `json:"pageIds,omitempty"`
	EvidenceIDs []string      `json:"evidenceIds,omitempty"`
}

type CoverageManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	RepositoryID  string          `json:"repositoryId"`
	Revision      string          `json:"revision"`
	Entries       []CoverageEntry `json:"entries"`
}

func BuildIndex(root, repositoryID, revision string, include, exclude []string) (Index, error) {
	return BuildIndexCached(root, repositoryID, revision, include, exclude, "", MaxEvidenceBytes)
}

func BuildIndexCached(root, repositoryID, revision string, include, exclude []string, cachePath string, maxBytes int64) (Index, error) {
	if revision == "" {
		revision = gitRevision(root)
	}
	if maxBytes <= 0 {
		maxBytes = MaxEvidenceBytes
	}
	paths, tracked, err := repositoryFiles(root)
	if err != nil {
		return Index{}, err
	}
	index := Index{SchemaVersion: SchemaVersion, RepositoryID: repositoryID, Revision: revision}
	cache := loadEvidenceCache(cachePath)
	objects := gitObjects(root)
	updatedCache := evidenceCache{Version: 1, Entries: map[string]cacheEntry{}}
	for path, entry := range cache.Entries {
		updatedCache.Entries[path] = entry
	}
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	for _, rel := range paths {
		if !includedPath(rel, include, exclude) {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > maxBytes {
			index.Skipped = append(index.Skipped, SkippedEvidence{Path: rel, Reason: "file exceeds evidence size limit"})
			continue
		}
		resolvedPath, pathErr := filepath.EvalSymlinks(path)
		if rootErr == nil && pathErr == nil {
			relative, relErr := filepath.Rel(resolvedRoot, resolvedPath)
			if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				index.Skipped = append(index.Skipped, SkippedEvidence{Path: rel, Reason: "symlink resolves outside repository root"})
				continue
			}
		}
		authority := AuthorityUntracked
		if tracked[rel] {
			authority = AuthorityTracked
		}
		entry, cached := cache.Entries[rel]
		if cached && cacheEntryValid(entry, info, objects[rel], authority) {
			index.CacheHits++
			updatedCache.Entries[rel] = entry
			if entry.Binary {
				index.Skipped = append(index.Skipped, SkippedEvidence{Path: rel, Reason: "binary evidence is excluded"})
				continue
			}
			index.References = append(index.References, makeReference(repositoryID, revision, rel, entry.Hash, entry.Lines, entry.EvidenceType, authority))
			continue
		}
		index.CacheMisses++
		content, err := os.ReadFile(path)
		if err != nil {
			index.Skipped = append(index.Skipped, SkippedEvidence{Path: rel, Reason: "file could not be read"})
			continue
		}
		binary := isBinary(content)
		hash := sha256.Sum256(content)
		contentHash := hex.EncodeToString(hash[:])
		entry = cacheEntry{Path: rel, Size: info.Size(), ModifiedNS: info.ModTime().UnixNano(), GitObject: objects[rel], Hash: contentHash, Lines: lineCount(content), Binary: binary, EvidenceType: classifyPath(rel)}
		updatedCache.Entries[rel] = entry
		if binary {
			index.Skipped = append(index.Skipped, SkippedEvidence{Path: rel, Reason: "binary evidence is excluded"})
			continue
		}
		index.References = append(index.References, makeReference(repositoryID, revision, rel, contentHash, entry.Lines, entry.EvidenceType, authority))
	}
	sort.Slice(index.References, func(i, j int) bool { return index.References[i].Path < index.References[j].Path })
	sort.Slice(index.Skipped, func(i, j int) bool { return index.Skipped[i].Path < index.Skipped[j].Path })
	if cachePath != "" {
		if err := saveEvidenceCache(cachePath, updatedCache); err != nil {
			return Index{}, err
		}
	}
	return index, nil
}

func makeReference(repositoryID, revision, path, contentHash string, lines int, evidenceType EvidenceType, authority EvidenceAuthority) EvidenceReference {
	idHash := sha256.Sum256([]byte(repositoryID + "\x00" + path + "\x00" + contentHash))
	return EvidenceReference{ID: "evidence:" + hex.EncodeToString(idHash[:]), RepositoryID: repositoryID, Revision: revision, Path: path, LineStart: 1, LineEnd: lines, EvidenceType: evidenceType, ContentHash: contentHash, Authority: authority}
}

func cacheEntryValid(entry cacheEntry, info os.FileInfo, gitObject string, authority EvidenceAuthority) bool {
	if entry.Path == "" || entry.Size != info.Size() {
		return false
	}
	if authority == AuthorityTracked {
		return gitObject != "" && gitObject == entry.GitObject
	}
	return entry.ModifiedNS == info.ModTime().UnixNano()
}

func loadEvidenceCache(path string) evidenceCache {
	cache := evidenceCache{Version: 1, Entries: map[string]cacheEntry{}}
	if path == "" {
		return cache
	}
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &cache) != nil || cache.Version != 1 || cache.Entries == nil {
		return evidenceCache{Version: 1, Entries: map[string]cacheEntry{}}
	}
	return cache
}

func saveEvidenceCache(path string, cache evidenceCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func AttachDocumentation(root string, index Index) (Index, error) {
	repositoryRoot := filepath.Dir(root)
	byPath := map[string]EvidenceReference{}
	for _, ref := range index.References {
		byPath[filepath.ToSlash(ref.Path)] = ref
	}
	var files []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".md") && d.Name() != "INSTRUCTIONS.md" {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return index, err
	}
	sort.Strings(files)
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return index, err
		}
		rel, _ := filepath.Rel(root, path)
		pageID := CanonicalPath(rel)
		for _, refPath := range sourceReferences(string(content)) {
			ref := resolveReference(root, repositoryRoot, path, refPath, byPath)
			if ref == nil {
				continue
			}
			index.Dependencies = append(index.Dependencies, EvidenceDependency{EvidenceID: ref.ID, PageID: pageID, SectionID: "Source References", EntryID: pageID + "#" + ref.ID})
		}
	}
	sort.Slice(index.Dependencies, func(i, j int) bool {
		if index.Dependencies[i].PageID == index.Dependencies[j].PageID {
			return index.Dependencies[i].EvidenceID < index.Dependencies[j].EvidenceID
		}
		return index.Dependencies[i].PageID < index.Dependencies[j].PageID
	})
	return index, nil
}

func BuildImpact(index Index, plan planner.ComponentPlan, previousRevision, revision string, changedPaths []string, fullScan bool, reason string) ChangeImpact {
	return BuildImpactWithPrevious(index, Index{}, plan, previousRevision, revision, changedPaths, fullScan, reason)
}

// BuildImpactWithPrevious retains dependencies from the previous successful
// index so deleted or renamed evidence can still invalidate its former pages.
func BuildImpactWithPrevious(index, previous Index, plan planner.ComponentPlan, previousRevision, revision string, changedPaths []string, fullScan bool, reason string) ChangeImpact {
	impact := ChangeImpact{SchemaVersion: SchemaVersion, RepositoryID: index.RepositoryID, PreviousRevision: previousRevision, Revision: revision, ChangedPaths: uniqueSorted(changedPaths), AffectedPages: map[string][]string{}, AffectedUnits: map[string][]string{}, FullScan: fullScan, Reason: reason}
	pageByUnit := map[string][]string{}
	for _, page := range plan.Pages {
		pagePath := filepath.ToSlash(page.Path)
		if fullScan {
			impact.AffectedPages[pagePath] = []string{"full-scope fallback"}
		}
		if page.OwnerUnit != "" {
			pageByUnit[page.OwnerUnit] = append(pageByUnit[page.OwnerUnit], pagePath)
		}
	}
	refPages := map[string][]string{}
	refByID := map[string]string{}
	for _, source := range []Index{previous, index} {
		for _, dep := range source.Dependencies {
			refPages[dep.EvidenceID] = append(refPages[dep.EvidenceID], dep.PageID)
		}
		for _, ref := range source.References {
			refByID[ref.ID] = filepath.ToSlash(ref.Path)
		}
	}
	for _, changed := range changedPaths {
		changed = filepath.ToSlash(filepath.Clean(changed))
		for evidenceID, path := range refByID {
			if changed != path && !strings.HasPrefix(changed, strings.TrimSuffix(path, "/")+"/") && !strings.HasPrefix(path, strings.TrimSuffix(changed, "/")+"/") {
				continue
			}
			for _, page := range refPages[evidenceID] {
				impact.AffectedPages[page] = append(impact.AffectedPages[page], "evidence:"+path)
			}
		}
		for _, unit := range plan.Units {
			for _, root := range unit.SourceRoots {
				root = strings.TrimSuffix(filepath.ToSlash(filepath.Clean(root)), "/")
				if root == "." || changed == root || strings.HasPrefix(changed, root+"/") {
					impact.AffectedUnits[unit.ID] = append(impact.AffectedUnits[unit.ID], "source:"+changed)
					for _, page := range pageByUnit[unit.ID] {
						impact.AffectedPages[page] = append(impact.AffectedPages[page], "unit:"+unit.ID)
					}
				}
			}
		}
	}
	for page, reasons := range impact.AffectedPages {
		impact.AffectedPages[page] = uniqueSorted(reasons)
	}
	for unit, reasons := range impact.AffectedUnits {
		impact.AffectedUnits[unit] = uniqueSorted(reasons)
	}
	return impact
}

func BuildCoverage(componentID string, plan planner.ComponentPlan, index Index) CoverageManifest {
	depsByPage := map[string][]string{}
	for _, dep := range index.Dependencies {
		depsByPage[dep.PageID] = append(depsByPage[dep.PageID], dep.EvidenceID)
	}
	manifest := CoverageManifest{SchemaVersion: SchemaVersion, RepositoryID: index.RepositoryID, Revision: index.Revision}
	for _, unit := range plan.Units {
		var pages, evidenceIDs []string
		for _, page := range plan.Pages {
			if page.OwnerUnit != unit.ID {
				continue
			}
			pageID := CanonicalPath(page.Path)
			pages = append(pages, pageID)
			evidenceIDs = append(evidenceIDs, depsByPage[pageID]...)
		}
		state := NotInspected
		if len(pages) > 0 {
			state = Documented
			if len(evidenceIDs) > 0 {
				state = Verified
			}
		}
		manifest.Entries = append(manifest.Entries, CoverageEntry{ID: componentID + ":unit:" + unit.ID, Scope: "unit", ComponentID: componentID, UnitID: unit.ID, State: state, PageIDs: uniqueSorted(pages), EvidenceIDs: uniqueSorted(evidenceIDs)})
	}
	for _, pack := range plan.Packs {
		var pages, evidenceIDs []string
		for _, page := range plan.Pages {
			if strings.HasPrefix(page.Path, "catalogs/") || strings.HasPrefix(page.Path, "platform/") {
				pageID := CanonicalPath(page.Path)
				pages = append(pages, pageID)
				evidenceIDs = append(evidenceIDs, depsByPage[pageID]...)
			}
		}
		state := NotApplicable
		if len(pages) > 0 {
			state = Documented
			if len(evidenceIDs) > 0 {
				state = Verified
			}
		}
		manifest.Entries = append(manifest.Entries, CoverageEntry{ID: componentID + ":pack:" + pack, Scope: "concern-pack", ComponentID: componentID, ConcernPack: pack, State: state, PageIDs: uniqueSorted(pages), EvidenceIDs: uniqueSorted(evidenceIDs)})
	}
	manifest.Entries = append(manifest.Entries, CoverageEntry{ID: componentID + ":component", Scope: "component", ComponentID: componentID, State: stateForPages(plan.Pages, depsByPage)})
	manifest.Entries = append(manifest.Entries, CoverageEntry{ID: index.RepositoryID + ":repository", Scope: "repository", State: stateForPages(plan.Pages, depsByPage)})
	sort.Slice(manifest.Entries, func(i, j int) bool { return manifest.Entries[i].ID < manifest.Entries[j].ID })
	return manifest
}

func stateForPages(pages []planner.PlannedPage, deps map[string][]string) CoverageState {
	if len(pages) == 0 {
		return NotInspected
	}
	for _, page := range pages {
		if len(deps[CanonicalPath(page.Path)]) > 0 {
			return Verified
		}
	}
	return Documented
}

func CanonicalPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	return strings.TrimSuffix(path, filepath.Ext(path))
}

func SaveJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Fingerprint(index Index) string {
	hash := sha256.New()
	for _, ref := range index.References {
		hash.Write([]byte(ref.Path))
		hash.Write([]byte{0})
		hash.Write([]byte(ref.ContentHash))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func LoadIndex(path string) (Index, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Index{}, err
	}
	var index Index
	if err := json.Unmarshal(b, &index); err != nil {
		return index, err
	}
	if index.SchemaVersion != SchemaVersion {
		return index, fmt.Errorf("unsupported evidence index schema version %d", index.SchemaVersion)
	}
	return index, nil
}

func LoadImpact(path string) (ChangeImpact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ChangeImpact{}, err
	}
	var impact ChangeImpact
	if err := json.Unmarshal(b, &impact); err != nil {
		return impact, err
	}
	if impact.SchemaVersion != SchemaVersion {
		return impact, fmt.Errorf("unsupported impact index schema version %d", impact.SchemaVersion)
	}
	return impact, nil
}

func LoadCoverage(path string) (CoverageManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return CoverageManifest{}, err
	}
	var coverage CoverageManifest
	if err := json.Unmarshal(b, &coverage); err != nil {
		return coverage, err
	}
	if coverage.SchemaVersion != SchemaVersion {
		return coverage, fmt.Errorf("unsupported coverage schema version %d", coverage.SchemaVersion)
	}
	return coverage, nil
}

func repositoryFiles(root string) ([]string, map[string]bool, error) {
	tracked := map[string]bool{}
	cmd := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard")
	b, err := cmd.Output()
	if err != nil {
		return walkFiles(root), tracked, nil
	}
	trackedBytes, _ := exec.Command("git", "-C", root, "ls-files", "-c").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(trackedBytes)), "\n") {
		if strings.TrimSpace(line) != "" {
			tracked[filepath.ToSlash(strings.TrimSpace(line))] = true
		}
	}
	set := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		line = filepath.ToSlash(strings.TrimSpace(line))
		if line != "" {
			set[line] = true
		}
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, tracked, nil
}

func gitObjects(root string) map[string]string {
	objects := map[string]string{}
	b, err := exec.Command("git", "-C", root, "ls-files", "--stage", "-z").Output()
	if err != nil {
		return objects
	}
	for _, item := range strings.Split(string(b), "\x00") {
		parts := strings.SplitN(item, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[0])
		if len(fields) >= 2 {
			objects[filepath.ToSlash(parts[1])] = fields[1]
		}
	}
	return objects
}

func walkFiles(root string) []string {
	var paths []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() && excludedDirectory(d.Name()) {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(root, path)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(paths)
	return paths
}

func includedPath(path string, include, exclude []string) bool {
	segments := strings.Split(filepath.ToSlash(path), "/")
	for _, segment := range segments[:len(segments)-1] {
		if excludedDirectory(segment) {
			return false
		}
	}
	if strings.HasPrefix(path, ".git/") {
		return false
	}
	if len(include) > 0 && !anyGlob(include, path) {
		return false
	}
	return !anyGlob(exclude, path)
}

func anyGlob(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if globMatch(pattern, path) {
			return true
		}
	}
	return false
}

func globMatch(pattern, path string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	quoted := regexp.QuoteMeta(pattern)
	quoted = strings.ReplaceAll(quoted, `\*\*`, `.*`)
	quoted = strings.ReplaceAll(quoted, `\*`, `[^/]*`)
	quoted = strings.ReplaceAll(quoted, `\?`, `[^/]`)
	matched, _ := regexp.MatchString("^"+quoted+"$", path)
	return matched || strings.HasSuffix(pattern, "/**") && strings.HasPrefix(path, strings.TrimSuffix(pattern, "**"))
}

func excludedDirectory(name string) bool {
	switch name {
	case ".git", ".wikiforge", "openwiki", "node_modules", "vendor", "generated", "target", "bin", "obj", ".venv", ".terraform":
		return true
	default:
		return false
	}
}

func isBinary(content []byte) bool {
	return strings.IndexByte(string(content), 0) >= 0
}

func lineCount(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	return strings.Count(string(content), "\n") + 1
}

func classifyPath(path string) EvidenceType {
	lower := strings.ToLower(path)
	if strings.Contains(lower, "test") || strings.Contains(lower, "spec") {
		return EvidenceTest
	}
	if strings.Contains(lower, "config") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".json") {
		return EvidenceConfig
	}
	return EvidenceSourceFile
}

func gitRevision(root string) string {
	b, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

var sourceSectionRE = regexp.MustCompile(`(?ms)^##\s+Source References\s*\n(.*)$`)
var backtickRE = regexp.MustCompile("`([^`]+)`")
var linkTargetRE = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

func sourceReferences(content string) []string {
	match := sourceSectionRE.FindStringSubmatch(strings.ReplaceAll(content, "\r\n", "\n"))
	if len(match) != 2 {
		return nil
	}
	section := match[1]
	if idx := strings.Index(section, "\n## "); idx >= 0 {
		section = section[:idx]
	}
	set := map[string]bool{}
	for _, item := range backtickRE.FindAllStringSubmatch(section, -1) {
		ref := cleanReference(item[1])
		if ref != "" {
			set[ref] = true
		}
	}
	for _, item := range linkTargetRE.FindAllStringSubmatch(section, -1) {
		ref := cleanReference(item[1])
		if ref != "" {
			set[ref] = true
		}
	}
	refs := make([]string, 0, len(set))
	for ref := range set {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}

func cleanReference(ref string) string {
	ref = strings.TrimSpace(strings.Split(ref, "#")[0])
	ref = strings.TrimSuffix(ref, ":")
	if ref == "" || strings.Contains(ref, "http://") || strings.Contains(ref, "https://") || strings.ContainsAny(ref, "{}") || strings.ContainsAny(ref, "*?") {
		return ""
	}
	return strings.TrimPrefix(filepath.ToSlash(ref), "./")
}

func resolveReference(documentRoot, repositoryRoot, documentPath, ref string, byPath map[string]EvidenceReference) *EvidenceReference {
	var candidates []string
	if strings.HasPrefix(ref, "/") {
		candidates = append(candidates, strings.TrimPrefix(ref, "/"))
	} else {
		if rel, err := filepath.Rel(documentRoot, filepath.Join(filepath.Dir(documentPath), filepath.FromSlash(ref))); err == nil {
			candidates = append(candidates, filepath.ToSlash(rel))
		}
		if rel, err := filepath.Rel(repositoryRoot, filepath.Join(filepath.Dir(documentPath), filepath.FromSlash(ref))); err == nil {
			candidates = append(candidates, filepath.ToSlash(rel))
		}
		candidates = append(candidates, filepath.ToSlash(filepath.Clean(ref)))
	}
	for _, candidate := range candidates {
		if refEvidence, ok := byPath[candidate]; ok {
			copy := refEvidence
			return &copy
		}
	}
	return nil
}

func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
