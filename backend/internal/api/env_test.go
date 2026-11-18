package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/config"
	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

const testKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://test.example:6443
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user:
    token: not-a-real-token
`

// testEnv is a Server on a real (temporary) store whose cluster runtimes
// use one fake clientset, with three users signed in:
// admin (global admin), alice (operator) and bob (viewer).
type testEnv struct {
	t       *testing.T
	cfg     *config.Config
	store   *store.Store
	srv     *Server
	handler http.Handler
	cs      *fake.Clientset
	dyn     *dynamicfake.FakeDynamicClient
	clock   *fakeClock
	cluster int64 // id of the "test" cluster
	tokens  map[string]string
	users   map[string]int64
	// apiServer, when set before a runtime is built, is the URL raw
	// requests (the /api/k8s passthrough) go to.
	apiServer string
}

func newTestEnv(t *testing.T, env map[string]string, plugins ...plugin.Plugin) *testEnv {
	t.Helper()
	dir := t.TempDir()
	vars := map[string]string{"KUBEBENCH_DB": filepath.Join(dir, "kubebench.db")}
	for k, v := range env {
		vars[k] = v
	}
	cfg, err := config.FromEnv(func(k string) (string, bool) { v, ok := vars[k]; return v, ok }, false)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	replicas := int32(2)
	cs := fake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			},
		},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "default"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-2", Namespace: "default"}},
	)
	dyn := plainDynamicClient()
	e := &testEnv{
		t: t, cfg: cfg, store: st, cs: cs, dyn: dyn,
		clock:  &fakeClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)},
		tokens: map[string]string{}, users: map[string]int64{},
	}
	manager := kube.NewManagerWith(func(c store.Cluster) (*kube.Runtime, error) {
		rt := kube.NewRuntimeForTest(c, cs, dyn)
		if e.apiServer != "" {
			rt.Config = &rest.Config{Host: e.apiServer}
		}
		return rt, nil
	})
	e.cluster, err = st.AddCluster(store.Cluster{Name: "test", Source: store.SourceKubeconfig, Kubeconfig: testKubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	e.srv = New(cfg, st, manager, BuildInfo{Version: "1.2.3", Commit: "abc1234", Date: "2026-11-14"}, plugins)
	e.srv.limiter.now = e.clock.Now
	e.srv.detect.now = e.clock.Now
	e.handler = e.srv.Handler(nil)

	e.addUser("admin", store.RoleAdmin, "")
	e.addUser("alice", store.RoleUser, store.ClusterRoleOperator)
	e.addUser("bob", store.RoleUser, store.ClusterRoleViewer)
	return e
}

const testPassword = "password-123"

// addUser creates a signed-in user with the test cluster active; a
// non-empty clusterRole assigns the cluster.
func (e *testEnv) addUser(name, role, clusterRole string) {
	e.t.Helper()
	id, err := e.store.CreateUser(name, testPassword, role, false)
	if err != nil {
		e.t.Fatal(err)
	}
	if clusterRole != "" {
		if err := e.store.SetAssignments(id, []store.Assignment{{ClusterID: e.cluster, Role: clusterRole}}); err != nil {
			e.t.Fatal(err)
		}
	}
	if err := e.store.SetActiveCluster(id, e.cluster); err != nil {
		e.t.Fatal(err)
	}
	token, err := e.store.CreateSession(id, time.Hour)
	if err != nil {
		e.t.Fatal(err)
	}
	e.users[name], e.tokens[name] = id, token
}

// do sends a request as user ("" = anonymous); a non-nil body is JSON.
func (e *testEnv) do(method, target, user string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.tokens[user]})
	}
	return e.serve(req)
}

func (e *testEnv) serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

// expect checks the status and, for errors, the §3.3 code.
func expect(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, status, rec.Body.String())
	}
	if code == "" {
		return
	}
	var body plugin.ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != code || body.Error == "" {
		t.Fatalf("error body = %s, want code %q", rec.Body.String(), code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("error Content-Type = %q", ct)
	}
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return v
}

// mutations lists the Kubernetes requests that change state.
func (e *testEnv) mutations() []k8stesting.Action {
	var out []k8stesting.Action
	for _, a := range e.cs.Actions() {
		switch a.GetVerb() {
		case "get", "list", "watch":
		default:
			out = append(out, a)
		}
	}
	return out
}

func (e *testEnv) audit() []store.AuditEntry {
	e.t.Helper()
	rows, err := e.store.ListAudit(store.AuditQuery{Limit: 100})
	if err != nil {
		e.t.Fatal(err)
	}
	return rows
}

func (e *testEnv) replicas() int32 {
	e.t.Helper()
	d, err := e.cs.AppsV1().Deployments("default").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil {
		e.t.Fatal(err)
	}
	return *d.Spec.Replicas
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// fakePlugin is a backend plugin with one route per access level.
type fakePlugin struct {
	id      string
	status  atomic.Pointer[plugin.Status]
	detects atomic.Int32
}

func newFakePlugin(id string) *fakePlugin {
	p := &fakePlugin{id: id}
	p.status.Store(&plugin.Status{Available: true})
	return p
}

func (p *fakePlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{ID: p.id, Name: "Fake", Version: "0.0.1", Description: "test plugin", Category: "tools"}
}

func (p *fakePlugin) Detect(context.Context, *kube.Runtime) plugin.Status {
	p.detects.Add(1)
	return *p.status.Load()
}

func (p *fakePlugin) Routes(r plugin.Router) {
	r.Handle(http.MethodGet, "/items", plugin.AccessRead, func(w http.ResponseWriter, _ *http.Request, c *plugin.Context) {
		plugin.WriteJSON(w, map[string]string{"user": c.User.Username, "cluster": c.Runtime.Cluster.Name})
	})
	r.Handle(http.MethodPost, "/items/{name}", plugin.AccessWrite|plugin.DryRunnable, func(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
		if !plugin.IsDryRun(r) {
			if err := c.KV.Set("item/"+r.PathValue("name"), "created"); err != nil {
				plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
				return
			}
		}
		plugin.WriteJSON(w, map[string]bool{"dryRun": plugin.IsDryRun(r)})
	})
	r.Handle(http.MethodGet, "/items/stored", plugin.AccessRead, func(w http.ResponseWriter, _ *http.Request, c *plugin.Context) {
		items, err := c.KV.List("item/")
		if err != nil {
			plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
			return
		}
		plugin.WriteJSON(w, items)
	})
	r.Handle(http.MethodPost, "/settings", plugin.AccessAdmin, func(w http.ResponseWriter, _ *http.Request, c *plugin.Context) {
		c.Audit("fake.settings", "settings/global", `{"extra":true}`)
		plugin.WriteJSON(w, map[string]bool{"ok": true})
	})
}
