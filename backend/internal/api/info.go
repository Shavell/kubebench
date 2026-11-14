package api

import (
	"context"
	"net/http"
	"os"
	"time"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// probeWait bounds how long /api/info waits for a fresh runtime's
// capability probe, so capabilities are not reported false merely because
// the probe is still running (§4.1).
const probeWait = 3 * time.Second

type clusterSummary struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Environment string `json:"environment"`
	ReadOnly    bool   `json:"readOnly"`
}

type features struct {
	PathKubeconfig  bool `json:"pathKubeconfig"`
	ExternalPlugins bool `json:"externalPlugins"`
}

type infoView struct {
	Version            string            `json:"version"`
	Commit             string            `json:"commit"`
	User               string            `json:"user"`
	Role               string            `json:"role"`
	MustChangePassword bool              `json:"mustChangePassword"`
	Context            string            `json:"context"`
	Server             string            `json:"server"`
	Cluster            *clusterSummary   `json:"cluster"`
	Permissions        permissions       `json:"permissions"`
	Capabilities       kube.Capabilities `json:"capabilities"`
	Features           features          `json:"features"`
	ClusterError       string            `json:"clusterError"`
}

// handleInfo describes the session for the app's boot (§4.1). It
// activates a cluster when none is usable (see selectCluster).
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request, c *call) {
	info := infoView{
		Version:            s.build.Version,
		Commit:             s.build.Commit,
		User:               c.user.Username,
		Role:               c.user.Role,
		MustChangePassword: c.user.MustChangePassword,
		Permissions:        permissionsFor(c.user, "", nil),
		Features: features{
			PathKubeconfig:  s.cfg.AllowPathKubeconfig,
			ExternalPlugins: isDir(s.cfg.PluginsDir),
		},
	}
	cluster, role, err := s.selectCluster(c.user)
	if err != nil {
		info.ClusterError = err.Error()
		plugin.WriteJSON(w, info)
		return
	}
	info.Context = cluster.Name
	info.Cluster = &clusterSummary{ID: cluster.ID, Name: cluster.Name, Environment: cluster.Environment, ReadOnly: cluster.ReadOnly}
	info.Permissions = permissionsFor(c.user, role, cluster)
	rt, err := s.manager.Runtime(cluster)
	if err != nil {
		info.ClusterError = "cluster " + cluster.Name + " is unavailable: " + err.Error()
		plugin.WriteJSON(w, info)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), probeWait)
	defer cancel()
	_ = rt.WaitProbed(ctx) // after the wait, an unfinished probe reports no capabilities
	info.Server = rt.Config.Host
	info.Capabilities = rt.Caps()
	plugin.WriteJSON(w, info)
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request, _ *call) {
	plugin.WriteJSON(w, s.build)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request, _ *call) {
	plugin.WriteJSON(w, map[string]string{"status": "ok"})
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
