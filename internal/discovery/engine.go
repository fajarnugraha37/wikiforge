package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/openwiki"
	"github.com/fajarnugraha37/wikiforge/internal/prompts"
)

type Engine struct {
	Config config.Config
	Runner openwiki.Runner
}

type RunMetrics struct {
	Stages       int                    `json:"stages"`
	Calls        int                    `json:"calls"`
	Retries      int                    `json:"retries"`
	CacheHit     bool                   `json:"cacheHit"`
	StageMetrics []DiscoveryStageMetric `json:"stageMetrics,omitempty"`
	Counts       DiscoveryCounts        `json:"counts"`
}

func (e Engine) Discover(ctx context.Context, component config.ComponentConfig, previous IdentityManifest) (Inventory, SemanticDiscovery, IdentityManifest, RunMetrics, error) {
	cacheDirectory := e.Config.Documentation.Evidence.CacheDirectory
	if cacheDirectory == "" {
		cacheDirectory = filepath.Join(e.Config.Workspace, ".wikiforge", "cache", "evidence")
	}
	cachePath := filepath.Join(cacheDirectory, sanitize(component.ID)+".json")
	inv, err := BuildInventory(component.WorkDir(), component.ID, "", cachePath, e.Config.Documentation.Evidence.Include, e.Config.Documentation.Evidence.Exclude, e.Config.Documentation.Evidence.MaxFileBytes)
	if err != nil {
		return Inventory{}, SemanticDiscovery{}, IdentityManifest{}, RunMetrics{}, err
	}
	result := SemanticDiscovery{SchemaVersion: SchemaVersion, ComponentID: component.ID, RepositoryID: component.ID, SourceRevision: inv.Revision, DiscoveryMode: e.Config.Documentation.Discovery.Mode, InventoryFingerprint: inv.Fingerprint, InventoryVersion: InventoryVersion, PromptVersion: PromptVersion, ModelID: e.Config.OpenWiki.ModelID}
	result.CacheFingerprint = CacheFingerprint(inv, e.Config, component)
	metrics := RunMetrics{}
	if e.Config.Documentation.Discovery.Mode != "hybrid" {
		result.Repository = RepositoryFinding{Profile: component.Profile, Confidence: "high", Status: StatusExplicitEnabled}
		result.Quality.Accepted = true
		result.Quality.AcceptedCount = len(e.Config.DocumentationUnits)
	} else {
		if e.Runner == nil {
			return inv, result, IdentityManifest{}, metrics, fmt.Errorf("hybrid discovery requires an OpenWiki runner")
		}
		stages := []string{"module-classification", "concern-flow-extraction", "semantic-synthesis"}
		stageOutputs := []StageOutput{}
		for stageIndex, stage := range stages {
			metrics.Stages++
			stageMetric := DiscoveryStageMetric{Name: stage, CacheHit: false}
			stageStarted := time.Now()
			batches := []Inventory{inv}
			if stageIndex < 2 {
				batches = inventoryBatches(inv, 256)
			}
			for batchIndex, batch := range batches {
				stageMetric.Batches++
				context := inventoryPrompt(batch)
				if stageIndex == 2 {
					context = stageContext(stageOutputs)
				}
				prompt, err := prompts.RenderTemplateValues("prompts/discovery/semantic-map.md", e.Config.Documentation.Language, component.ID, map[string]string{
					"STAGE": stage, "COMPONENT_ID": component.ID, "PROFILE": component.Profile, "INVENTORY": context, "BATCH": fmt.Sprintf("%d/%d", batchIndex+1, len(batches)),
				})
				if err != nil {
					return inv, result, IdentityManifest{}, metrics, err
				}
				var output string
				for attempt := 0; ; attempt++ {
					output, err = e.Runner.Run(openwiki.WithRunLabel(ctx, component.ID+"/discovery/"+stage), component.WorkDir(), "discovery", prompt)
					metrics.Calls++
					stageMetric.Calls++
					if err == nil || attempt >= e.Config.Execution.MaxProcessRetries || openwiki.IsNonRetryableError(err) {
						break
					}
					metrics.Retries++
					stageMetric.Retries++
				}
				if err != nil {
					return inv, result, IdentityManifest{}, metrics, fmt.Errorf("discovery stage %s: %w", stage, err)
				}
				stageResult, err := decodeStageOutput(output, stage)
				if err != nil {
					return inv, result, IdentityManifest{}, metrics, fmt.Errorf("discovery stage %s returned invalid strict JSON: %w", stage, err)
				}
				stageOutputs = append(stageOutputs, stageResult)
				mergeStage(&result, stageResult)
			}
			stageMetric.DurationMillis = time.Since(stageStarted).Milliseconds()
			metrics.StageMetrics = append(metrics.StageMetrics, stageMetric)
		}
	}
	if latest := gitRevision(component.WorkDir()); latest != "" && inv.Revision != "" && latest != inv.Revision {
		return inv, result, IdentityManifest{}, metrics, fmt.Errorf("source revision changed during discovery from %s to %s", inv.Revision, latest)
	}
	latestInventory, err := BuildInventory(component.WorkDir(), component.ID, "", cachePath, e.Config.Documentation.Evidence.Include, e.Config.Documentation.Evidence.Exclude, e.Config.Documentation.Evidence.MaxFileBytes)
	if err != nil {
		return inv, result, IdentityManifest{}, metrics, fmt.Errorf("recheck inventory after discovery: %w", err)
	}
	if latestInventory.Fingerprint != inv.Fingerprint || latestInventory.Revision != inv.Revision {
		return inv, result, IdentityManifest{}, metrics, fmt.Errorf("source inventory changed during discovery")
	}
	result = normalize(result)
	stripUntrustedIdentities(&result)
	applyExplicitIdentities(e.Config.DocumentationUnits, component.ID, &result)
	identities := ResolveIdentities(component.ID, previous, &result)
	if err := Validate(inv, result); err != nil {
		return inv, result, identities, metrics, err
	}
	result.Quality = quality(result)
	metrics.Counts = DiscoveryCounts{Modules: len(result.Modules), Domains: len(result.Domains), Flows: len(result.Flows), Concerns: len(result.Concerns), Ownership: len(result.Ownership), Relationships: len(result.Relationships)}
	for _, conflict := range result.Conflicts {
		if conflictBlocksPromotion(conflict, result, e.Config.Documentation.Discovery.OnConflict) {
			return inv, result, identities, metrics, fmt.Errorf("semantic discovery conflict %s blocks promotion: %s", conflict.Dimension, conflict.Message)
		}
	}
	if !result.Quality.Accepted && e.Config.Documentation.Discovery.Required {
		return inv, result, identities, metrics, fmt.Errorf("semantic discovery produced no accepted findings for component %s", component.ID)
	}
	return inv, result, identities, metrics, nil
}

func stripUntrustedIdentities(result *SemanticDiscovery) {
	clear := func(base *FindingBase) {
		base.ID = ""
		if base.Provenance == "explicit" {
			base.Provenance = ""
		}
	}
	for i := range result.Modules {
		clear(&result.Modules[i].FindingBase)
	}
	for i := range result.Domains {
		clear(&result.Domains[i].FindingBase)
	}
	for i := range result.Flows {
		clear(&result.Flows[i].FindingBase)
	}
	for i := range result.Concerns {
		clear(&result.Concerns[i].FindingBase)
	}
	for i := range result.Ownership {
		clear(&result.Ownership[i].FindingBase)
	}
	for i := range result.Relationships {
		clear(&result.Relationships[i].FindingBase)
	}
}

func gitRevision(root string) string {
	b, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func applyExplicitIdentities(units []config.DocumentationUnitConfig, componentID string, result *SemanticDiscovery) {
	for _, unit := range units {
		if unit.Component != componentID {
			continue
		}
		if unit.Kind != "domain" && unit.Kind != "flow" {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(unit.Domain))
		if name == "" {
			name = strings.ToLower(strings.TrimSpace(unit.ID))
		}
		if unit.Kind == "domain" {
			for i := range result.Domains {
				candidate := strings.ToLower(strings.TrimSpace(result.Domains[i].Candidate.Name))
				if candidate == name || normalizeID(candidate) == normalizeID(name) {
					result.Domains[i].ID = unit.ID
					result.Domains[i].Provenance = "explicit"
					result.Domains[i].Status = StatusExplicitEnabled
				}
			}
		}
		if unit.Kind == "flow" {
			for i := range result.Flows {
				candidate := strings.ToLower(strings.TrimSpace(result.Flows[i].Candidate.Name))
				if candidate == name || normalizeID(candidate) == normalizeID(name) {
					result.Flows[i].ID = unit.ID
					result.Flows[i].Provenance = "explicit"
					result.Flows[i].Status = StatusExplicitEnabled
				}
			}
		}
	}
}

func conflictBlocksPromotion(conflict ConflictFinding, result SemanticDiscovery, policy config.ConflictConfig) bool {
	fail := conflict.Dimension == "domainIdentity" && policy.DomainIdentity == "fail" || conflict.Dimension == "sourceOwnership" && policy.SourceOwnership == "fail"
	if !fail {
		return false
	}
	if len(conflict.SubjectIDs) == 0 {
		return true
	}
	for _, subject := range conflict.SubjectIDs {
		for _, domain := range result.Domains {
			if domain.ID == subject && (domain.Status == StatusObserved || domain.Status == StatusExplicitEnabled) {
				return true
			}
		}
		for _, module := range result.Modules {
			if module.ID == subject && (module.Status == StatusObserved || module.Status == StatusExplicitEnabled) {
				return true
			}
		}
	}
	return false
}

func ValidatePromotion(result SemanticDiscovery, policy config.ConflictConfig) error {
	for _, conflict := range result.Conflicts {
		if conflictBlocksPromotion(conflict, result, policy) {
			return fmt.Errorf("semantic discovery conflict %s blocks promotion: %s", conflict.Dimension, conflict.Message)
		}
	}
	return nil
}

func CacheFingerprint(inv Inventory, cfg config.Config, component config.ComponentConfig) string {
	b, _ := json.Marshal(struct {
		Inventory    string
		Discovery    config.DiscoveryConfig
		Component    config.ComponentConfig
		Units        []config.DocumentationUnitConfig
		Include      []string
		Exclude      []string
		MaxFileBytes int64
		Command      string
		Args         []string
		Model        string
		Prompt       string
		Environment  map[string]string
	}{inv.Fingerprint, cfg.Documentation.Discovery, component, cfg.DocumentationUnits, cfg.Documentation.Evidence.Include, cfg.Documentation.Evidence.Exclude, cfg.Documentation.Evidence.MaxFileBytes, cfg.OpenWiki.Command, cfg.OpenWiki.Args, cfg.OpenWiki.ModelID, PromptVersion, cfg.OpenWiki.Environment})
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

func mergeStage(dst *SemanticDiscovery, src StageOutput) {
	if src.Repository.Profile != "" {
		dst.Repository = src.Repository
	}
	dst.Modules = mergeModules(dst.Modules, src.Modules)
	dst.Domains = mergeDomains(dst.Domains, src.Domains)
	dst.Flows = mergeFlows(dst.Flows, src.Flows)
	dst.Concerns = mergeConcerns(dst.Concerns, src.Concerns)
	dst.Ownership = mergeOwnership(dst.Ownership, src.Ownership)
	dst.Relationships = mergeRelationships(dst.Relationships, src.Relationships)
	dst.Conflicts = mergeConflicts(dst.Conflicts, src.Conflicts)
	dst.Unknowns = mergeUnknowns(dst.Unknowns, src.Unknowns)
}

func mergeConcerns(dst, src []ConcernFinding) []ConcernFinding {
	for _, item := range src {
		found := false
		for i := range dst {
			if item.Concern != "" && strings.EqualFold(dst[i].Concern, item.Concern) {
				dst[i].FindingBase = mergeFindingBase(dst[i].FindingBase, item.FindingBase)
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, item)
		}
	}
	return dst
}
func mergeOwnership(dst, src []OwnershipFinding) []OwnershipFinding {
	for _, item := range src {
		found := false
		for i := range dst {
			if item.SubjectID == dst[i].SubjectID {
				dst[i].FindingBase = mergeFindingBase(dst[i].FindingBase, item.FindingBase)
				dst[i].Owners = uniqueStrings(append(dst[i].Owners, item.Owners...))
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, item)
		}
	}
	return dst
}
func mergeRelationships(dst, src []RelationshipFinding) []RelationshipFinding {
	for _, item := range src {
		found := false
		for i := range dst {
			old := &dst[i]
			if old.FromID == item.FromID && old.ToID == item.ToID && old.Kind == item.Kind {
				old.FindingBase = mergeFindingBase(old.FindingBase, item.FindingBase)
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, item)
		}
	}
	return dst
}
func mergeConflicts(dst, src []ConflictFinding) []ConflictFinding {
	for _, item := range src {
		found := false
		for i := range dst {
			if item.ID != "" && item.ID == dst[i].ID || item.Dimension == dst[i].Dimension && item.Message == dst[i].Message {
				dst[i].EvidenceIDs = uniqueStrings(append(dst[i].EvidenceIDs, item.EvidenceIDs...))
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, item)
		}
	}
	return dst
}
func mergeUnknowns(dst, src []UnknownFinding) []UnknownFinding {
	for _, item := range src {
		found := false
		for i := range dst {
			if item.Dimension == dst[i].Dimension && item.Subject == dst[i].Subject {
				dst[i].EvidenceIDs = uniqueStrings(append(dst[i].EvidenceIDs, item.EvidenceIDs...))
				if item.Status != "" {
					dst[i].Status = item.Status
				}
				if item.Reason != "" {
					dst[i].Reason = item.Reason
				}
				if item.Description != "" {
					dst[i].Description = item.Description
				}
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, item)
		}
	}
	return dst
}

func mergeModules(dst, src []ModuleFinding) []ModuleFinding {
	for _, item := range src {
		replaced := false
		for i := range dst {
			if item.Candidate.CandidateKey != "" && dst[i].Candidate.CandidateKey == item.Candidate.CandidateKey {
				dst[i].FindingBase = mergeFindingBase(dst[i].FindingBase, item.FindingBase)
				if item.Role != "" {
					dst[i].Role = item.Role
				}
				dst[i].SourceRoots = uniqueStrings(append(dst[i].SourceRoots, item.SourceRoots...))
				dst[i].Domains = uniqueStrings(append(dst[i].Domains, item.Domains...))
				replaced = true
				break
			}
		}
		if !replaced {
			dst = append(dst, item)
		}
	}
	return dst
}
func mergeDomains(dst, src []DomainFinding) []DomainFinding {
	for _, item := range src {
		replaced := false
		for i := range dst {
			if item.Candidate.CandidateKey != "" && dst[i].Candidate.CandidateKey == item.Candidate.CandidateKey {
				dst[i].FindingBase = mergeFindingBase(dst[i].FindingBase, item.FindingBase)
				if item.Subdomain != "" {
					dst[i].Subdomain = item.Subdomain
				}
				if item.BoundedContext != "" {
					dst[i].BoundedContext = item.BoundedContext
				}
				dst[i].ModuleIDs = uniqueStrings(append(dst[i].ModuleIDs, item.ModuleIDs...))
				dst[i].SourceRoots = uniqueStrings(append(dst[i].SourceRoots, item.SourceRoots...))
				dst[i].Owners = uniqueStrings(append(dst[i].Owners, item.Owners...))
				if item.Criticality != "" {
					dst[i].Criticality = item.Criticality
				}
				replaced = true
				break
			}
		}
		if !replaced {
			dst = append(dst, item)
		}
	}
	return dst
}
func mergeFlows(dst, src []FlowFinding) []FlowFinding {
	for _, item := range src {
		replaced := false
		for i := range dst {
			if item.Candidate.CandidateKey != "" && dst[i].Candidate.CandidateKey == item.Candidate.CandidateKey {
				dst[i].FindingBase = mergeFindingBase(dst[i].FindingBase, item.FindingBase)
				if item.Triggers != nil {
					dst[i].Triggers = uniqueStrings(append(dst[i].Triggers, item.Triggers...))
				}
				dst[i].Actors = uniqueStrings(append(dst[i].Actors, item.Actors...))
				dst[i].States = uniqueStrings(append(dst[i].States, item.States...))
				dst[i].ModuleIDs = uniqueStrings(append(dst[i].ModuleIDs, item.ModuleIDs...))
				dst[i].SourceRoots = uniqueStrings(append(dst[i].SourceRoots, item.SourceRoots...))
				replaced = true
				break
			}
		}
		if !replaced {
			dst = append(dst, item)
		}
	}
	return dst
}

func mergeFindingBase(left, right FindingBase) FindingBase {
	merged := right
	if merged.Candidate.Name == "" {
		merged.Candidate.Name = left.Candidate.Name
	}
	if merged.Candidate.CandidateKey == "" {
		merged.Candidate.CandidateKey = left.Candidate.CandidateKey
	}
	if merged.Status == "" {
		merged.Status = left.Status
	}
	if merged.Confidence == "" {
		merged.Confidence = left.Confidence
	}
	if merged.Description == "" {
		merged.Description = left.Description
	}
	if merged.Candidate.Description == "" {
		merged.Candidate.Description = left.Candidate.Description
	}
	if merged.Provenance == "" {
		merged.Provenance = left.Provenance
	}
	merged.EvidenceIDs = uniqueStrings(append(left.EvidenceIDs, right.EvidenceIDs...))
	merged.Candidate.EvidenceIDs = uniqueStrings(append(left.Candidate.EvidenceIDs, right.Candidate.EvidenceIDs...))
	return merged
}

func inventoryBatches(inv Inventory, batchSize int) []Inventory {
	if batchSize <= 0 || len(inv.Evidence) <= batchSize {
		return []Inventory{inv}
	}
	var batches []Inventory
	for start := 0; start < len(inv.Evidence); start += batchSize {
		end := start + batchSize
		if end > len(inv.Evidence) {
			end = len(inv.Evidence)
		}
		batch := inv
		batch.Evidence = append([]EvidenceReference{}, inv.Evidence[start:end]...)
		batch.Files = make([]string, 0, len(batch.Evidence))
		for _, ref := range batch.Evidence {
			batch.Files = append(batch.Files, ref.Path)
		}
		batch.Signals = nil
		for _, signal := range inv.Signals {
			for _, ref := range batch.Evidence {
				if signal.EvidenceID == ref.ID {
					batch.Signals = append(batch.Signals, signal)
					break
				}
			}
		}
		batches = append(batches, batch)
	}
	return batches
}

func stageContext(outputs []StageOutput) string { b, _ := json.Marshal(outputs); return string(b) }

func decodeStageOutput(output, expectedStage string) (StageOutput, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return StageOutput{}, fmt.Errorf("empty output")
	}
	objects := []string{trimmed}
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		objects = jsonObjectCandidates(trimmed)
	}
	var valid []StageOutput
	var firstErr error
	for _, object := range objects {
		var candidate StageOutput
		dec := json.NewDecoder(strings.NewReader(object))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&candidate); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		var trailing any
		if err := dec.Decode(&trailing); err != io.EOF {
			continue
		}
		if candidate.SchemaVersion == SchemaVersion && candidate.Stage == expectedStage {
			valid = append(valid, candidate)
		}
	}
	if len(valid) == 1 {
		return valid[0], nil
	}
	if len(valid) > 1 {
		return StageOutput{}, fmt.Errorf("output contains multiple valid %s JSON objects", expectedStage)
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("no valid schema=%d stage=%q object found", SchemaVersion, expectedStage)
	}
	return StageOutput{}, firstErr
}

func jsonObjectCandidates(value string) []string {
	var candidates []string
	start := -1
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(value); i++ {
		char := value[i]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				candidates = append(candidates, value[start:i+1])
				start = -1
			}
		}
	}
	return candidates
}

func normalize(d SemanticDiscovery) SemanticDiscovery {
	dedup := func(ids []string) []string {
		set := map[string]bool{}
		out := []string{}
		for _, id := range ids {
			if id != "" && !set[id] {
				set[id] = true
				out = append(out, id)
			}
		}
		sort.Strings(out)
		return out
	}
	for i := range d.Modules {
		d.Modules[i].EvidenceIDs = dedup(append(d.Modules[i].EvidenceIDs, d.Modules[i].Candidate.EvidenceIDs...))
		d.Modules[i].Candidate.EvidenceIDs = d.Modules[i].EvidenceIDs
	}
	for i := range d.Domains {
		d.Domains[i].EvidenceIDs = dedup(append(d.Domains[i].EvidenceIDs, d.Domains[i].Candidate.EvidenceIDs...))
		d.Domains[i].Candidate.EvidenceIDs = d.Domains[i].EvidenceIDs
	}
	for i := range d.Flows {
		d.Flows[i].EvidenceIDs = dedup(append(d.Flows[i].EvidenceIDs, d.Flows[i].Candidate.EvidenceIDs...))
		d.Flows[i].Candidate.EvidenceIDs = d.Flows[i].EvidenceIDs
	}
	for i := range d.Concerns {
		d.Concerns[i].EvidenceIDs = dedup(append(d.Concerns[i].EvidenceIDs, d.Concerns[i].Candidate.EvidenceIDs...))
		d.Concerns[i].Candidate.EvidenceIDs = d.Concerns[i].EvidenceIDs
	}
	for i := range d.Unknowns {
		if d.Unknowns[i].Candidate != nil {
			d.Unknowns[i].EvidenceIDs = dedup(append(d.Unknowns[i].EvidenceIDs, d.Unknowns[i].Candidate.EvidenceIDs...))
			d.Unknowns[i].Candidate.EvidenceIDs = d.Unknowns[i].EvidenceIDs
			if d.Unknowns[i].Subject == "" {
				d.Unknowns[i].Subject = d.Unknowns[i].Candidate.Name
			}
		}
		if d.Unknowns[i].Dimension == "" {
			d.Unknowns[i].Dimension = "unknown"
		}
		if d.Unknowns[i].Subject == "" {
			d.Unknowns[i].Subject = "unspecified"
		}
		if d.Unknowns[i].Reason == "" {
			d.Unknowns[i].Reason = "model returned an unresolved finding without a reason"
		}
	}
	return d
}

func uniqueStrings(values []string) []string {
	set := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value != "" && !set[value] {
			set[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func quality(d SemanticDiscovery) QualityResult {
	q := QualityResult{}
	count := func(status string) {
		switch status {
		case StatusUncertain:
			q.UncertainCount++
		case StatusConflicting:
			q.ConflictingCount++
		case StatusUnknown:
			q.UnknownCount++
		}
	}
	for _, x := range d.Modules {
		if x.Status == StatusObserved || x.Status == StatusExplicitEnabled {
			q.AcceptedCount++
		}
		count(x.Status)
	}
	for _, x := range d.Domains {
		if x.Status == StatusObserved || x.Status == StatusExplicitEnabled {
			q.AcceptedCount++
		}
		count(x.Status)
	}
	for _, x := range d.Flows {
		if x.Status == StatusObserved || x.Status == StatusExplicitEnabled {
			q.AcceptedCount++
		}
		count(x.Status)
	}
	for _, x := range d.Concerns {
		if x.Status == StatusObserved || x.Status == StatusExplicitEnabled {
			q.AcceptedCount++
		}
		count(x.Status)
	}
	q.Accepted = d.DiscoveryMode == "explicit" || d.DiscoveryMode == "disabled" || q.AcceptedCount > 0 || len(d.Unknowns) > 0
	q.UnknownCount = len(d.Unknowns)
	return q
}

func inventoryPrompt(inv Inventory) string { b, _ := json.Marshal(inv); return string(b) }
func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "component"
	}
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(s)
}
