package orchestrator

import (
	"context"
	"path/filepath"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/discovery"
)

func (o *Orchestrator) ensureSemanticDiscovery(ctx context.Context, component config.ComponentConfig, artifacts evidenceArtifacts) (discovery.Inventory, discovery.SemanticDiscovery, discovery.IdentityManifest, discovery.RunMetrics, error) {
	cacheDirectory := o.Config.Documentation.Evidence.CacheDirectory
	if cacheDirectory == "" {
		cacheDirectory = filepath.Join(o.Config.Workspace, ".wikiforge", "cache", "evidence")
	}
	cachePath := filepath.Join(cacheDirectory, sanitizeArtifactName(component.ID)+".json")
	inv, err := discovery.BuildInventory(component.WorkDir(), component.ID, "", cachePath, o.Config.Documentation.Evidence.Include, o.Config.Documentation.Evidence.Exclude, o.Config.Documentation.Evidence.MaxFileBytes)
	if err != nil {
		return discovery.Inventory{}, discovery.SemanticDiscovery{}, discovery.IdentityManifest{}, discovery.RunMetrics{}, err
	}
	var previous discovery.IdentityManifest
	_ = discovery.LoadJSON(artifacts.Identities, &previous)
	if o.Config.Documentation.Discovery.ReuseCache {
		var cached discovery.SemanticDiscovery
		if err := discovery.LoadJSON(artifacts.Semantic, &cached); err == nil && cached.InventoryFingerprint == inv.Fingerprint && cached.CacheFingerprint == discovery.CacheFingerprint(inv, o.Config, component) && cached.SourceRevision == inv.Revision && cached.ComponentID == component.ID {
			if err := discovery.Validate(inv, cached); err == nil && discovery.ValidatePromotion(cached, o.Config.Documentation.Discovery.OnConflict) == nil {
				var cachedIdentities discovery.IdentityManifest
				if err := discovery.LoadJSON(artifacts.Identities, &cachedIdentities); err == nil {
					if err := discovery.SaveJSON(artifacts.Inventory, inv); err != nil {
						return inv, cached, cachedIdentities, discovery.RunMetrics{CacheHit: true}, err
					}
					return inv, cached, cachedIdentities, discovery.RunMetrics{CacheHit: true, StageMetrics: []discovery.DiscoveryStageMetric{{Name: "semantic-discovery", CacheHit: true}}, Counts: discovery.DiscoveryCounts{Modules: len(cached.Modules), Domains: len(cached.Domains), Flows: len(cached.Flows), Concerns: len(cached.Concerns), Ownership: len(cached.Ownership), Relationships: len(cached.Relationships)}}, nil
				}
			}
		}
	}
	engine := discovery.Engine{Config: o.Config, Runner: o.Runner}
	inv, semantic, identities, metrics, err := engine.Discover(ctx, component, previous)
	if err != nil {
		return inv, semantic, identities, metrics, err
	}
	if err := discovery.SaveJSON(artifacts.Inventory, inv); err != nil {
		return inv, semantic, identities, metrics, err
	}
	if err := discovery.SaveJSON(artifacts.Semantic, semantic); err != nil {
		return inv, semantic, identities, metrics, err
	}
	if err := discovery.SaveJSON(artifacts.Identities, identities); err != nil {
		return inv, semantic, identities, metrics, err
	}
	return inv, semantic, identities, metrics, nil
}
