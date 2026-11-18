package main

import (
	"errors"
	"testing"

	"kubebench/internal/store"
)

func TestSeedGroupMappingsOnceAndAudited(t *testing.T) {
	cfg := testConfig(t, map[string]string{
		"KUBEBENCH_AUTH_GROUP_MAPPINGS": `[{"group":"kb-admins","role":"admin"},{"group":"ops","role":"viewer","cluster":"prod"}]`,
	})
	st := openStore(t, cfg)
	for range 2 {
		if err := seedGroupMappings(cfg, st); err != nil {
			t.Fatal(err)
		}
	}
	mappings, _ := st.ListGroupMappings()
	if len(mappings) != 2 {
		t.Fatalf("mappings = %+v", mappings)
	}
	rows, _ := st.ListAudit(store.AuditQuery{Limit: 10, Action: "auth.mappings.seed"})
	if len(rows) != 1 || rows[0].Detail != `{"count":2}` {
		t.Fatalf("audit = %+v", rows)
	}
}

func TestSeedGroupMappingsRefusesInvalidOnes(t *testing.T) {
	cfg := testConfig(t, map[string]string{"KUBEBENCH_AUTH_GROUP_MAPPINGS": `[{"group":"ops","role":"viewer"}]`})
	err := seedGroupMappings(cfg, openStore(t, cfg))
	if cerr, ok := errors.AsType[*configError](err); !ok || cerr.key != "KUBEBENCH_AUTH_GROUP_MAPPINGS" {
		t.Fatalf("err = %v", err)
	}
}
