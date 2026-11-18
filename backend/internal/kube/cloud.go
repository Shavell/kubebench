package kube

import (
	"context"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/cloud"
	"kubebench/internal/openshift"
)

const (
	// cloudTTL is how long a detection is served before a read refreshes
	// it in the background: an upgrade changes the version, not the
	// runtime.
	cloudTTL = time.Hour
	// cloudNodeSample bounds how many nodes detection reads.
	cloudNodeSample = 20
)

// cloudState is the runtime's cached provider detection (§7.12).
type cloudState struct {
	done       chan struct{} // closed once the first detection finished
	detection  cloud.Detection
	at         time.Time
	refreshing bool
}

// Cloud returns the detected provider and version. ok is false while the
// first detection is still running; a detection older than cloudTTL is
// returned as is and refreshed in the background.
func (r *Runtime) Cloud() (d cloud.Detection, ok bool) {
	select {
	case <-r.platform.done:
	default:
		return cloud.Detection{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(r.platform.at) > cloudTTL && !r.platform.refreshing {
		r.platform.refreshing = true
		go r.refreshCloud()
	}
	return r.platform.detection, true
}

// WaitCloud blocks until the first detection finished, or returns ctx's
// error when ctx ends first.
func (r *Runtime) WaitCloud(ctx context.Context) error { return wait(ctx, r.platform.done) }

func (r *Runtime) detectCloudOnce() {
	defer close(r.platform.done)
	r.refreshCloud()
}

func (r *Runtime) refreshCloud() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	d := r.DetectCloud(ctx)
	r.mu.Lock()
	r.platform.detection, r.platform.at, r.platform.refreshing = d, time.Now(), false
	r.mu.Unlock()
	log.Printf("[cloud] cluster=%s provider=%s cloud=%s version=%s product=%s", r.Cluster.Name, d.Provider, d.Cloud, d.Version, d.ProductVersion)
}

// DetectCloud reads the signals and detects the provider: the API server
// URL, its version, a sample of the nodes and OpenShift's ClusterVersion.
// A signal the credential may not read (e.g. nodes) is left out.
func (r *Runtime) DetectCloud(ctx context.Context) cloud.Detection {
	signals := cloud.Signals{Server: r.Config.Host}
	if v, err := r.CS.Discovery().ServerVersion(); err == nil {
		signals.GitVersion = v.GitVersion
	}
	if nodes, err := r.CS.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: cloudNodeSample}); err == nil {
		for _, n := range nodes.Items {
			signals.Nodes = append(signals.Nodes, cloud.Node{ProviderID: n.Spec.ProviderID, Labels: n.Labels})
		}
	}
	// OpenShift's own version decides its support window (§7.15); other
	// clusters answer 404 for the ClusterVersion. (Test runtimes may have
	// no dynamic client.)
	if r.Dyn != nil {
		signals.OpenShiftVersion = openshift.Version(ctx, r.Dyn)
	}
	return cloud.Detect(signals)
}
