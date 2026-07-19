package discovery

import "github.com/fajarnugraha37/wikiforge/internal/model"

const (
	SchemaVersion          = model.DiscoverySchemaVersion
	InventoryVersion       = "inventory-v2"
	PromptVersion          = "semantic-map-v2"
	StatusObserved         = model.StatusObserved
	StatusNotObserved      = model.StatusNotObserved
	StatusUncertain        = model.StatusUncertain
	StatusConflicting      = model.StatusConflicting
	StatusExplicitEnabled  = model.StatusExplicitEnabled
	StatusExplicitDisabled = model.StatusExplicitDisabled
	StatusNotApplicable    = model.StatusNotApplicable
	StatusUnknown          = model.StatusUnknown
)

type EvidenceLocator = model.EvidenceLocator
type EvidenceReference = model.EvidenceReference
type SemanticCandidate = model.SemanticCandidate
type FindingBase = model.FindingBase
type RepositoryFinding = model.RepositoryFinding
type ModuleFinding = model.ModuleFinding
type DomainFinding = model.DomainFinding
type FlowFinding = model.FlowFinding
type ConcernFinding = model.ConcernFinding
type OwnershipFinding = model.OwnershipFinding
type RelationshipFinding = model.RelationshipFinding
type ConflictFinding = model.ConflictFinding
type UnknownFinding = model.UnknownFinding
type SemanticDiscovery = model.SemanticDiscovery
type DiscoveryStageMetric = model.DiscoveryStageMetric
type DiscoveryCounts = model.DiscoveryCounts
type QualityResult = model.QualityResult
type IdentityMapping = model.IdentityMapping
type IdentityManifest = model.IdentityManifest
type StageOutput = model.StageOutput
