package config

import (
	"strings"
	"testing"
)

func TestMonitoringConfig(t *testing.T) {
	c, err := FromEnv(env(nil), false)
	if err != nil || c.Monitoring != (MonitoringConfig{}) {
		t.Fatalf("defaults = %+v, %v", c.Monitoring, err)
	}
	c, err = FromEnv(env(map[string]string{
		"KUBEBENCH_OPENSHIFT_MONITORING_ADDRESS":  "127.0.0.1:18443",
		"KUBEBENCH_OPENSHIFT_MONITORING_INSECURE": "true",
	}), false)
	if err != nil || c.Monitoring != (MonitoringConfig{Address: "127.0.0.1:18443", InsecureSkipVerify: true}) {
		t.Fatalf("set = %+v, %v", c.Monitoring, err)
	}
	for _, bad := range []string{"127.0.0.1", ":8443", "router:https", "router:0"} {
		_, err := FromEnv(env(map[string]string{"KUBEBENCH_OPENSHIFT_MONITORING_ADDRESS": bad}), false)
		if err == nil || !strings.Contains(err.Error(), "KUBEBENCH_OPENSHIFT_MONITORING_ADDRESS:") {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
	if _, err := FromEnv(env(map[string]string{"KUBEBENCH_OPENSHIFT_MONITORING_INSECURE": "maybe"}), false); err == nil {
		t.Error("a non-boolean insecure flag must be refused")
	}
}
