package discovery

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

var statuses = map[string]bool{StatusObserved: true, StatusNotObserved: true, StatusUncertain: true, StatusConflicting: true, StatusExplicitEnabled: true, StatusExplicitDisabled: true, StatusNotApplicable: true, StatusUnknown: true}
var confidences = map[string]bool{"high": true, "medium": true, "low": true}
var moduleRoles = map[string]bool{"business": true, "technical": true, "test": true, "deployment": true, "mixed": true, "unknown": true}

func Validate(inv Inventory, result SemanticDiscovery) error {
	if result.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported semantic discovery schema version %d", result.SchemaVersion)
	}
	if result.ComponentID == "" || result.RepositoryID == "" || result.InventoryFingerprint != inv.Fingerprint {
		return fmt.Errorf("semantic discovery identity or inventory fingerprint does not match inventory")
	}
	if result.InventoryVersion != InventoryVersion {
		return fmt.Errorf("unsupported inventory version %q", result.InventoryVersion)
	}
	if result.PromptVersion != PromptVersion {
		return fmt.Errorf("unsupported prompt version %q", result.PromptVersion)
	}
	if result.DiscoveryMode != "hybrid" && result.DiscoveryMode != "explicit" && result.DiscoveryMode != "disabled" {
		return fmt.Errorf("unsupported discovery mode %q", result.DiscoveryMode)
	}
	byEvidence := map[string]EvidenceReference{}
	knownPaths := map[string]bool{}
	for _, path := range inv.Files {
		knownPaths[filepath.ToSlash(filepath.Clean(path))] = true
	}
	for _, ref := range inv.Evidence {
		clean := filepath.ToSlash(filepath.Clean(ref.Path))
		if clean == "." || filepath.IsAbs(ref.Path) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("inventory contains unsafe evidence path %q", ref.Path)
		}
		if ref.Locator.LineStart < 0 || ref.Locator.LineEnd < 0 || (ref.Locator.LineEnd > 0 && ref.Locator.LineStart > ref.Locator.LineEnd) {
			return fmt.Errorf("evidence %q has invalid locator", ref.ID)
		}
		if ref.ID == "" || ref.ContentHash == "" || !knownPaths[clean] {
			return fmt.Errorf("inventory evidence %q is incomplete or absent from inventory files", ref.ID)
		}
		if _, exists := byEvidence[ref.ID]; exists {
			return fmt.Errorf("inventory contains duplicate evidence id %q", ref.ID)
		}
		byEvidence[ref.ID] = ref
	}
	checkBase := func(kind string, base FindingBase) error {
		if !statuses[base.Status] {
			return fmt.Errorf("%s has invalid status %q", kind, base.Status)
		}
		if !confidences[base.Confidence] {
			return fmt.Errorf("%s has invalid confidence %q", kind, base.Confidence)
		}
		ids := append([]string{}, base.EvidenceIDs...)
		ids = append(ids, base.Candidate.EvidenceIDs...)
		if base.Status == StatusNotApplicable && base.Provenance != "explicit" && base.Provenance != "exhaustive-deterministic" {
			return fmt.Errorf("%s not-applicable requires explicit or exhaustive deterministic provenance", kind)
		}
		if base.Status == StatusObserved && len(ids) == 0 {
			return fmt.Errorf("%s observed finding requires evidence", kind)
		}
		if (base.Status == StatusObserved || base.Status == StatusExplicitEnabled) && strings.TrimSpace(base.Candidate.Name) == "" {
			return fmt.Errorf("%s accepted finding requires candidate name", kind)
		}
		if strings.TrimSpace(base.Candidate.CandidateKey) == "" && strings.TrimSpace(base.Candidate.Name) == "" {
			return fmt.Errorf("%s requires candidate key or name", kind)
		}
		for _, id := range ids {
			if _, ok := byEvidence[id]; !ok {
				return fmt.Errorf("%s references unknown evidence id %q", kind, id)
			}
		}
		return nil
	}
	if !statuses[result.Repository.Status] || !confidences[result.Repository.Confidence] {
		return fmt.Errorf("repository has invalid status or confidence")
	}
	for _, id := range result.Repository.EvidenceIDs {
		if _, ok := byEvidence[id]; !ok {
			return fmt.Errorf("repository references unknown evidence id %q", id)
		}
	}
	for _, item := range result.Modules {
		if err := checkBase("module", item.FindingBase); err != nil {
			return err
		}
		if acceptedStatus(item.Status) && !moduleRoles[item.Role] {
			return fmt.Errorf("module has invalid or missing role %q", item.Role)
		}
		if err := checkRoots("module", item.SourceRoots, inv.Files); err != nil {
			return err
		}
	}
	for _, item := range result.Domains {
		if err := checkBase("domain", item.FindingBase); err != nil {
			return err
		}
		if err := checkRoots("domain", item.SourceRoots, inv.Files); err != nil {
			return err
		}
	}
	for _, item := range result.Flows {
		if err := checkBase("flow", item.FindingBase); err != nil {
			return err
		}
		if err := checkRoots("flow", item.SourceRoots, inv.Files); err != nil {
			return err
		}
		if item.Status == StatusObserved && len(item.Triggers) == 0 {
			return fmt.Errorf("flow observed finding requires at least one trigger")
		}
	}
	for _, item := range result.Concerns {
		if err := checkBase("concern", item.FindingBase); err != nil {
			return err
		}
	}
	for _, item := range result.Ownership {
		if err := checkBase("ownership", item.FindingBase); err != nil {
			return err
		}
	}
	for _, item := range result.Relationships {
		if err := checkBase("relationship", item.FindingBase); err != nil {
			return err
		}
	}
	for _, item := range result.Unknowns {
		if item.Status != "" && !statuses[item.Status] {
			return fmt.Errorf("unknown finding has invalid status %q", item.Status)
		}
		if item.Confidence != "" && !confidences[item.Confidence] {
			return fmt.Errorf("unknown finding has invalid confidence %q", item.Confidence)
		}
		for _, id := range item.EvidenceIDs {
			if _, ok := byEvidence[id]; !ok {
				return fmt.Errorf("unknown finding references unknown evidence id %q", id)
			}
		}
	}
	ids := map[string]bool{}
	candidateKeys := map[string]bool{}
	for _, list := range [][]FindingBase{basesModules(result.Modules), basesDomains(result.Domains), basesFlows(result.Flows), basesConcerns(result.Concerns), basesOwnership(result.Ownership), basesRelationships(result.Relationships)} {
		for _, base := range list {
			if base.Candidate.CandidateKey != "" {
				if candidateKeys[base.Candidate.CandidateKey] {
					return fmt.Errorf("duplicate semantic candidate key %q", base.Candidate.CandidateKey)
				}
				candidateKeys[base.Candidate.CandidateKey] = true
			}
			if base.ID != "" {
				if ids[base.ID] {
					return fmt.Errorf("duplicate semantic finding id %q", base.ID)
				}
				ids[base.ID] = true
			}
		}
	}
	for _, rel := range result.Relationships {
		if rel.FromID != "" && !ids[rel.FromID] {
			return fmt.Errorf("relationship references missing fromId %q", rel.FromID)
		}
		if rel.ToID != "" && !ids[rel.ToID] {
			return fmt.Errorf("relationship references missing toId %q", rel.ToID)
		}
	}
	for _, item := range result.Ownership {
		if item.SubjectID != "" && !ids[item.SubjectID] {
			return fmt.Errorf("ownership references missing subjectId %q", item.SubjectID)
		}
	}
	for _, item := range result.Domains {
		for _, moduleID := range item.ModuleIDs {
			if moduleID != "" && !ids[moduleID] {
				return fmt.Errorf("domain references missing moduleId %q", moduleID)
			}
		}
	}
	for _, item := range result.Flows {
		for _, moduleID := range item.ModuleIDs {
			if moduleID != "" && !ids[moduleID] {
				return fmt.Errorf("flow references missing moduleId %q", moduleID)
			}
		}
	}
	for _, item := range result.Modules {
		for _, domainID := range item.Domains {
			if domainID != "" && !ids[domainID] {
				return fmt.Errorf("module references missing domainId %q", domainID)
			}
		}
	}
	for _, item := range result.Conflicts {
		for _, id := range item.EvidenceIDs {
			if _, ok := byEvidence[id]; !ok {
				return fmt.Errorf("conflict references unknown evidence id %q", id)
			}
		}
		if item.Dimension == "" || item.Message == "" {
			return fmt.Errorf("conflict requires dimension and message")
		}
		for _, subjectID := range item.SubjectIDs {
			if subjectID != "" && !ids[subjectID] {
				return fmt.Errorf("conflict references missing subjectId %q", subjectID)
			}
		}
	}
	if err := validateDomainRoots(result.Domains); err != nil {
		return err
	}
	if err := validateCandidateNames(result); err != nil {
		return err
	}
	if err := validateModuleAssignments(result); err != nil {
		return err
	}
	if err := validateRelationshipShape(result.Relationships); err != nil {
		return err
	}
	return nil
}

func validateCandidateNames(result SemanticDiscovery) error {
	seen := map[string]string{}
	check := func(base FindingBase) error {
		name := normalizeID(base.Candidate.Name)
		if name == "" {
			return nil
		}
		key := base.Candidate.CandidateKey
		if prior, ok := seen[name]; ok && prior != key && key != "" && prior != "" {
			return fmt.Errorf("incompatible duplicate semantic name %q", base.Candidate.Name)
		}
		seen[name] = key
		return nil
	}
	for _, item := range result.Modules {
		if err := check(item.FindingBase); err != nil {
			return err
		}
	}
	for _, item := range result.Domains {
		if err := check(item.FindingBase); err != nil {
			return err
		}
	}
	for _, item := range result.Flows {
		if err := check(item.FindingBase); err != nil {
			return err
		}
	}
	for _, item := range result.Concerns {
		if err := check(item.FindingBase); err != nil {
			return err
		}
	}
	return nil
}

func validateModuleAssignments(result SemanticDiscovery) error {
	owners := map[string]string{}
	for _, domain := range result.Domains {
		if domain.Status != StatusObserved && domain.Status != StatusExplicitEnabled {
			continue
		}
		for _, module := range domain.ModuleIDs {
			if prior, ok := owners[module]; ok && prior != domain.ID && prior != "" {
				return fmt.Errorf("module %q has conflicting domain assignments %q and %q", module, prior, domain.ID)
			}
			owners[module] = domain.ID
		}
	}
	return nil
}

func validateDomainRoots(domains []DomainFinding) error {
	for i := range domains {
		for j := i + 1; j < len(domains); j++ {
			if !acceptedStatus(domains[i].Status) || !acceptedStatus(domains[j].Status) {
				continue
			}
			for _, left := range domains[i].SourceRoots {
				for _, right := range domains[j].SourceRoots {
					l, r := filepath.ToSlash(filepath.Clean(left)), filepath.ToSlash(filepath.Clean(right))
					if l == "." || r == "." || l == "" || r == "" {
						continue
					}
					if l == r || strings.HasPrefix(l, r+"/") || strings.HasPrefix(r, l+"/") {
						return fmt.Errorf("domain source roots overlap: %q and %q", left, right)
					}
				}
			}
		}
	}
	return nil
}

func acceptedStatus(status string) bool {
	return status == StatusObserved || status == StatusExplicitEnabled
}

func validateRelationshipShape(relationships []RelationshipFinding) error {
	for _, rel := range relationships {
		if strings.TrimSpace(rel.FromID) == "" || strings.TrimSpace(rel.ToID) == "" || strings.TrimSpace(rel.Kind) == "" {
			return fmt.Errorf("relationship requires fromId, toId, and kind")
		}
	}
	return nil
}

func checkRoots(kind string, roots, files []string) error {
	for _, root := range roots {
		clean := filepath.ToSlash(filepath.Clean(root))
		if clean == "." || clean == "" {
			continue
		}
		if filepath.IsAbs(root) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("%s has unsafe source root %q", kind, root)
		}
		found := false
		for _, file := range files {
			if file == clean || strings.HasPrefix(file, clean+"/") {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s source root %q is absent from deterministic inventory", kind, clean)
		}
	}
	return nil
}

func ResolveIdentities(componentID string, previous IdentityManifest, result *SemanticDiscovery) IdentityManifest {
	used := map[string]bool{}
	assigned := map[string]bool{}
	resolved := map[string]string{}
	ambiguous := map[string]bool{}
	addResolved := func(reference, id string) {
		if reference == "" || ambiguous[reference] {
			return
		}
		if prior, ok := resolved[reference]; ok && prior != id {
			resolved[reference] = ""
			ambiguous[reference] = true
			return
		}
		resolved[reference] = id
	}
	for _, mapping := range previous.Mappings {
		used[mapping.ID] = true
	}
	mappings := []IdentityMapping{}
	resolve := func(base *FindingBase) {
		if base.Candidate.Name == "" {
			return
		}
		id := ""
		matchedPrevious := false
		precedence := "normalized-name"
		if base.ID != "" && base.Provenance == "explicit" {
			id = base.ID
			precedence = "explicit-unit-id"
		}
		for _, old := range previous.Mappings {
			if id == "" && previousAccepted(old) && overlap(old.EvidenceIDs, base.EvidenceIDs) > 0 && overlap(old.EvidenceIDs, base.EvidenceIDs) >= sufficientOverlap(old.EvidenceIDs, base.EvidenceIDs) {
				id = old.ID
				matchedPrevious = true
				precedence = "previous-accepted-overlap"
				break
			}
		}
		if matchedPrevious && assigned[id] {
			matchedPrevious = false
		}
		if id == "" {
			id = normalizeID(base.Candidate.Name)
		}
		if id == "" {
			id = "finding"
		}
		if (used[id] || assigned[id]) && !matchedPrevious && precedence != "explicit-unit-id" {
			baseID := id
			for n := 2; used[id]; n++ {
				id = fmt.Sprintf("%s-%d", baseID, n)
			}
		}
		used[id] = true
		assigned[id] = true
		base.ID = id
		addResolved(base.Candidate.CandidateKey, id)
		addResolved(normalizeID(base.Candidate.Name), id)
		addResolved(id, id)
		mappings = append(mappings, IdentityMapping{CandidateKey: base.Candidate.CandidateKey, ID: id, Name: base.Candidate.Name, EvidenceIDs: append([]string{}, base.EvidenceIDs...), Precedence: precedence, Accepted: base.Status == StatusObserved || base.Status == StatusExplicitEnabled})
	}
	for i := range result.Modules {
		resolve(&result.Modules[i].FindingBase)
	}
	for i := range result.Domains {
		resolve(&result.Domains[i].FindingBase)
	}
	for i := range result.Flows {
		resolve(&result.Flows[i].FindingBase)
	}
	for i := range result.Concerns {
		resolve(&result.Concerns[i].FindingBase)
	}
	for i := range result.Ownership {
		resolve(&result.Ownership[i].FindingBase)
	}
	for i := range result.Relationships {
		resolve(&result.Relationships[i].FindingBase)
	}
	for i := range result.Relationships {
		result.Relationships[i].FromID = resolveReference(result.Relationships[i].FromID, resolved)
		result.Relationships[i].ToID = resolveReference(result.Relationships[i].ToID, resolved)
	}
	for i := range result.Domains {
		for j, ref := range result.Domains[i].ModuleIDs {
			result.Domains[i].ModuleIDs[j] = resolveReference(ref, resolved)
		}
	}
	for i := range result.Flows {
		for j, ref := range result.Flows[i].ModuleIDs {
			result.Flows[i].ModuleIDs[j] = resolveReference(ref, resolved)
		}
	}
	for i := range result.Modules {
		for j, ref := range result.Modules[i].Domains {
			result.Modules[i].Domains[j] = resolveReference(ref, resolved)
		}
	}
	for i := range result.Ownership {
		result.Ownership[i].SubjectID = resolveReference(result.Ownership[i].SubjectID, resolved)
	}
	for i := range result.Conflicts {
		for j, subject := range result.Conflicts[i].SubjectIDs {
			result.Conflicts[i].SubjectIDs[j] = resolveReference(subject, resolved)
		}
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].ID < mappings[j].ID })
	return IdentityManifest{SchemaVersion: SchemaVersion, ComponentID: componentID, Mappings: mappings}
}

func resolveReference(value string, resolved map[string]string) string {
	if id, ok := resolved[value]; ok {
		if id == "" {
			return "ambiguous:" + value
		}
		return id
	}
	if id, ok := resolved[normalizeID(value)]; ok {
		if id == "" {
			return "ambiguous:" + value
		}
		return id
	}
	return value
}

func previousAccepted(mapping IdentityMapping) bool {
	return mapping.Accepted || mapping.Precedence == ""
}
func sufficientOverlap(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) == 1 || len(b) == 1 {
		return 1
	}
	return 2
}

func normalizeID(value string) string {
	value = strings.ToLower(value)
	return strings.Trim(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r == ' ' || r == '_' || r == '/' || r == '-' {
			return '-'
		}
		return -1
	}, value), "-")
}
func overlap(a, b []string) int {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	n := 0
	for _, x := range b {
		if set[x] {
			n++
		}
	}
	return n
}
func basesModules(v []ModuleFinding) []FindingBase {
	out := make([]FindingBase, len(v))
	for i := range v {
		out[i] = v[i].FindingBase
	}
	return out
}
func basesDomains(v []DomainFinding) []FindingBase {
	out := make([]FindingBase, len(v))
	for i := range v {
		out[i] = v[i].FindingBase
	}
	return out
}
func basesFlows(v []FlowFinding) []FindingBase {
	out := make([]FindingBase, len(v))
	for i := range v {
		out[i] = v[i].FindingBase
	}
	return out
}
func basesConcerns(v []ConcernFinding) []FindingBase {
	out := make([]FindingBase, len(v))
	for i := range v {
		out[i] = v[i].FindingBase
	}
	return out
}
func basesOwnership(v []OwnershipFinding) []FindingBase {
	out := make([]FindingBase, len(v))
	for i := range v {
		out[i] = v[i].FindingBase
	}
	return out
}
func basesRelationships(v []RelationshipFinding) []FindingBase {
	out := make([]FindingBase, len(v))
	for i := range v {
		out[i] = v[i].FindingBase
	}
	return out
}
