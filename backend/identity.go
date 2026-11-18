package main

import (
	"fmt"
	"log"

	"kubebench/internal/config"
	"kubebench/internal/store"
)

// seedGroupMappings stores the group mappings of KUBEBENCH_AUTH_GROUP_MAPPINGS
// on the first start that has them (§3.6); from then on admins manage the
// mappings in Settings > Authentication. The seeding is audited. An
// invalid mapping is a configuration error.
func seedGroupMappings(cfg *config.Config, st *store.Store) error {
	if len(cfg.Auth.Mappings) == 0 {
		return nil
	}
	mappings := make([]store.GroupMapping, 0, len(cfg.Auth.Mappings))
	for _, m := range cfg.Auth.Mappings {
		mappings = append(mappings, store.GroupMapping{Group: m.Group, Source: m.Source, Role: m.Role, Cluster: m.Cluster})
	}
	seeded, err := st.SeedGroupMappings(mappings)
	if err != nil {
		return &configError{key: "KUBEBENCH_AUTH_GROUP_MAPPINGS", msg: err.Error()}
	}
	if !seeded {
		return nil
	}
	log.Printf("[auth] seeded %d group mappings from KUBEBENCH_AUTH_GROUP_MAPPINGS", len(mappings))
	return st.InsertAudit(store.AuditEntry{
		Username: "kubebench",
		Action:   "auth.mappings.seed",
		Target:   "mapping",
		Detail:   fmt.Sprintf(`{"count":%d}`, len(mappings)),
	})
}

// logIdentityConfig reports the external sign-in setup at startup, never
// its secrets.
func logIdentityConfig(cfg *config.Config) {
	if o := cfg.Auth.OIDC; o != nil {
		log.Printf("[auth] OIDC sign-in with %s (client %s)", o.Issuer, o.ClientID)
		if cfg.PublicURL == "" {
			log.Printf("[auth] KUBEBENCH_PUBLIC_URL is not set: the OIDC redirect URL is derived from each request's host")
		}
	}
	if cfg.Auth.External() {
		log.Printf("[auth] local accounts keep working; the last local admin cannot be removed (break-glass)")
	}
}
