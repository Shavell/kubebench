package api

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"kubebench/internal/cloud"
	"kubebench/internal/cloudauth"
	"kubebench/internal/kube"
	"kubebench/internal/store"
)

// platformView is where a cluster runs and whether its version is still
// supported there (§7.12; src/data/types.ts ClusterPlatform). Detected is
// false when only the API server URL was looked at, because the cluster's
// runtime is not built yet: then the version is "" and support unknown.
type platformView struct {
	Provider string        `json:"provider"`
	Cloud    string        `json:"cloud"`
	Version  string        `json:"version"`
	Support  cloud.Support `json:"support"`
	Detected bool          `json:"detected"`
}

func newPlatformView(d cloud.Detection, detected bool, now time.Time) *platformView {
	return &platformView{
		Provider: d.Provider, Cloud: d.Cloud, Version: d.Version, Detected: detected,
		Support: cloud.VersionSupport(d.Provider, d.Version, now),
	}
}

// platformOf describes a cluster from its runtime's detection when the
// runtime is built and detected, else from its server URL alone. It never
// builds a runtime: listing the inventory must not connect to every
// cluster.
func (s *Server) platformOf(c *store.Cluster, server string) *platformView {
	if rt := s.manager.Peek(c); rt != nil {
		if d, ok := rt.Cloud(); ok {
			return newPlatformView(d, true, time.Now())
		}
	}
	return newPlatformView(cloud.ProviderFromServer(server), false, time.Now())
}

// activePlatform describes the active cluster's runtime, waiting for its
// first detection like probedCaps waits for the capability probe.
func activePlatform(ctx context.Context, rt *kube.Runtime) *platformView {
	ctx, cancel := context.WithTimeout(ctx, probeWait)
	defer cancel()
	_ = rt.WaitCloud(ctx)
	if d, ok := rt.Cloud(); ok {
		return newPlatformView(d, true, time.Now())
	}
	return newPlatformView(cloud.ProviderFromServer(rt.Config.Host), false, time.Now())
}

// cloudAuthOf is the cloud whose identity signs in to a stored kubeconfig
// cluster in process (its user's exec stanza is a recognised one), or "".
func cloudAuthOf(c *store.Cluster) string {
	if c.Source != store.SourceKubeconfig {
		return ""
	}
	user, err := contextUser(c)
	if err != nil {
		return ""
	}
	return cloudauth.ProviderFor(user)
}

// contextUser is the user entry of a kubeconfig cluster's context (nil
// when the context names none).
func contextUser(c *store.Cluster) (*clientcmdapi.AuthInfo, error) {
	kc, err := clientcmd.Load([]byte(c.Kubeconfig))
	if err != nil {
		return nil, err
	}
	name := c.Context
	if name == "" {
		name = kc.CurrentContext
	}
	if ctx, ok := kc.Contexts[name]; ok {
		return kc.AuthInfos[ctx.AuthInfo], nil
	}
	return nil, nil
}

// decodeCredentials validates the credentials of an add or edit request
// for the cluster's kubeconfig context (§7.12). raw is the request's
// "credentials" member: absent (nil, no change), JSON null (remove them:
// "" with remove set) or an object.
func decodeCredentials(raw json.RawMessage, c *store.Cluster) (encoded string, remove bool, err error) {
	if raw == nil {
		return "", false, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", true, nil
	}
	var creds cloudauth.Credentials
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&creds); err != nil {
		return "", false, badRequest("invalid credentials: %v", err)
	}
	if err := creds.Validate(); err != nil {
		return "", false, badRequest("invalid credentials: %v", err)
	}
	if c.Source != store.SourceKubeconfig {
		return "", false, badRequest("cloud credentials apply to kubeconfig clusters only")
	}
	user, err := contextUser(c)
	if err != nil {
		return "", false, badRequest("invalid kubeconfig: %v", err)
	}
	if err := cloudauth.CheckCredentials(user, creds.Provider); err != nil {
		return "", false, badRequest("%v", err)
	}
	return creds.Encode(), false, nil
}
