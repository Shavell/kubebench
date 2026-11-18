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
	// Platform is null until the cluster's runtime is built (§7.12).
	Platform *platformView `json:"platform"`
}

type features struct {
	PathKubeconfig  bool `json:"pathKubeconfig"`
	ExternalPlugins bool `json:"externalPlugins"`
	// AmbientCloudIdentity: pasted EKS, GKE and AKS kubeconfigs may sign
	// in with the server's own cloud identity (§7.12).
	AmbientCloudIdentity bool `json:"ambientCloudIdentity"`
}

type infoView struct {
	Version            string            `json:"version"`
	Commit             string            `json:"commit"`
	User               string            `json:"user"`
	Role               string            `json:"role"`
	MustChangePassword bool              `json:"mustChangePassword"`
	AuthSource         string            `json:"authSource"`
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
		AuthSource:         c.user.Source,
		Permissions:        permissionsFor(c.user, "", nil),
		Features: features{
			PathKubeconfig:       s.cfg.AllowPathKubeconfig,
			ExternalPlugins:      isDir(s.cfg.PluginsDir),
			AmbientCloudIdentity: s.cfg.AllowAmbientCloudIdentity,
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
	info.Server = rt.Config.Host
	// The probe and cloud detection run side by side: one deadline for both.
	wait, cancel := context.WithTimeout(r.Context(), probeWait)
	defer cancel()
	info.Capabilities = probedCaps(wait, rt)
	info.Cluster.Platform = activePlatform(wait, rt)
	plugin.WriteJSON(w, info)
}

// probedCaps returns rt's capabilities once its probe finished, waiting at
// most probeWait; a probe still unfinished after that reports none.
func probedCaps(ctx context.Context, rt *kube.Runtime) kube.Capabilities {
	ctx, cancel := context.WithTimeout(ctx, probeWait)
	defer cancel()
	_ = rt.WaitProbed(ctx)
	return rt.Caps()
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
