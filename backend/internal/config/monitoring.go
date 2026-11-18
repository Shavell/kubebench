package config

import "net"

// MonitoringConfig is how KubeBench reaches OpenShift monitoring's Thanos
// Querier. By default it needs no settings: in a pod KubeBench uses the
// thanos-querier Service and the service CA, elsewhere the thanos-querier
// Route and the cluster's default ingress CA.
type MonitoringConfig struct {
	// Address (host:port, KUBEBENCH_OPENSHIFT_MONITORING_ADDRESS) is
	// dialled instead of the Thanos Querier's host name, which still names
	// the TLS server: for a router that the Route's host does not resolve
	// to from where KubeBench runs.
	Address string
	// InsecureSkipVerify (KUBEBENCH_OPENSHIFT_MONITORING_INSECURE) accepts
	// any certificate of the Thanos Querier. Off by default.
	InsecureSkipVerify bool
}

func (p *parser) monitoring() MonitoringConfig {
	m := MonitoringConfig{
		Address:            p.str("KUBEBENCH_OPENSHIFT_MONITORING_ADDRESS", ""),
		InsecureSkipVerify: p.boolean("KUBEBENCH_OPENSHIFT_MONITORING_INSECURE", false),
	}
	if m.Address != "" {
		if host, port, err := net.SplitHostPort(m.Address); err != nil || host == "" || !validPort(port) {
			p.fail("KUBEBENCH_OPENSHIFT_MONITORING_ADDRESS", "must be host:port, e.g. 127.0.0.1:8443")
		}
	}
	return m
}
