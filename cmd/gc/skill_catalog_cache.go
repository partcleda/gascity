package main

import (
	"sync"

	"github.com/gastownhall/gascity/internal/materialize"
)

// Transient filesystem errors in loadSharedSkillCatalog used to silently
// drop skill entries from FingerprintExtra for one tick, which flipped
// CoreFingerprint hashes and drained every live session in a config-drift
// storm. The cache preserves the last successful catalog for each
// (cityPath, rigName) so a single failed load reuses the prior result
// instead of emitting a degraded fingerprint. A subsequent successful
// load overwrites the cache, so real catalog edits still propagate.
var skillCatalogCache = struct {
	sync.Mutex
	city map[string]materialize.CityCatalog             // cityPath -> catalog
	rig  map[string]map[string]materialize.CityCatalog // cityPath -> rigName -> catalog
}{
	city: map[string]materialize.CityCatalog{},
	rig:  map[string]map[string]materialize.CityCatalog{},
}

// cachedCityCatalog returns the last successfully loaded city catalog for
// cityPath, or (zero, false) if none has been cached yet.
func cachedCityCatalog(cityPath string) (materialize.CityCatalog, bool) {
	skillCatalogCache.Lock()
	defer skillCatalogCache.Unlock()
	c, ok := skillCatalogCache.city[cityPath]
	return c, ok
}

// setCachedCityCatalog stores the catalog as the last successful load for
// cityPath. Callers should only call this on load success.
func setCachedCityCatalog(cityPath string, cat materialize.CityCatalog) {
	skillCatalogCache.Lock()
	defer skillCatalogCache.Unlock()
	skillCatalogCache.city[cityPath] = cat
}

// cachedRigCatalog returns the last successfully loaded rig catalog, or
// (zero, false) if none has been cached yet for this (cityPath, rigName).
func cachedRigCatalog(cityPath, rigName string) (materialize.CityCatalog, bool) {
	skillCatalogCache.Lock()
	defer skillCatalogCache.Unlock()
	byRig, ok := skillCatalogCache.rig[cityPath]
	if !ok {
		return materialize.CityCatalog{}, false
	}
	c, ok := byRig[rigName]
	return c, ok
}

// setCachedRigCatalog stores the catalog as the last successful load for
// (cityPath, rigName). Callers should only call this on load success.
func setCachedRigCatalog(cityPath, rigName string, cat materialize.CityCatalog) {
	skillCatalogCache.Lock()
	defer skillCatalogCache.Unlock()
	byRig, ok := skillCatalogCache.rig[cityPath]
	if !ok {
		byRig = map[string]materialize.CityCatalog{}
		skillCatalogCache.rig[cityPath] = byRig
	}
	byRig[rigName] = cat
}
