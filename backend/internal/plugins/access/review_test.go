package access

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	authv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/plugin"
)

// answerReviews makes the fake API server answer SubjectAccessReviews with
// answer and records the specs it was asked.
func answerReviews(cs *fake.Clientset, answer func(authv1.SubjectAccessReviewSpec) authv1.SubjectAccessReviewStatus) *[]authv1.SubjectAccessReviewSpec {
	var asked []authv1.SubjectAccessReviewSpec
	cs.PrependReactor("create", "subjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		sar := a.(k8stesting.CreateAction).GetObject().(*authv1.SubjectAccessReview).DeepCopy()
		asked = append(asked, sar.Spec)
		sar.Status = answer(sar.Spec)
		return true, sar, nil
	})
	return &asked
}

func TestReview(t *testing.T) {
	cs := fake.NewClientset(fixture()...)
	asked := answerReviews(cs, func(spec authv1.SubjectAccessReviewSpec) authv1.SubjectAccessReviewStatus {
		if spec.ResourceAttributes != nil && spec.ResourceAttributes.Resource == "secrets" {
			return authv1.SubjectAccessReviewStatus{Allowed: true, Reason: `RBAC: allowed by RoleBinding "argocd-server/argocd"`}
		}
		return authv1.SubjectAccessReviewStatus{}
	})
	e := newEnvWith(t, cs)

	body := expect(t, e.do(http.MethodPost, "/review", map[string]any{
		"subject": map[string]string{"kind": "ServiceAccount", "namespace": "argocd", "name": "argocd-server"},
		"verb":    "list", "resource": "secrets", "namespace": "argocd",
	}), http.StatusOK, "")
	res := decode[reviewResponse](t, body)
	if !res.Allowed || res.Reason == "" || len(res.Grants) != 1 || res.Grants[0].Binding.Name != "argocd-server" {
		t.Fatalf("review = %+v", res)
	}
	spec := (*asked)[0]
	if spec.User != "system:serviceaccount:argocd:argocd-server" ||
		!slices.Equal(spec.Groups, []string{"system:serviceaccounts", "system:serviceaccounts:argocd", "system:authenticated"}) ||
		*spec.ResourceAttributes != (authv1.ResourceAttributes{Verb: "list", Resource: "secrets", Namespace: "argocd"}) {
		t.Errorf("asked %+v", spec)
	}

	// A group and a non-resource URL: KubeBench's evaluation is reported
	// next to the server's answer, which wins.
	res = decode[reviewResponse](t, expect(t, e.do(http.MethodPost, "/review", map[string]any{
		"subject": map[string]string{"kind": "Group", "name": "system:authenticated"},
		"verb":    "get", "path": "/healthz",
	}), http.StatusOK, ""))
	if res.Allowed || len(res.Grants) != 1 || res.Grants[0].Binding.Name != "system:discovery" || res.Grants[0].Via != "system:authenticated" {
		t.Errorf("group review = %+v", res)
	}
	if spec := (*asked)[1]; spec.User != "" || spec.NonResourceAttributes == nil || spec.NonResourceAttributes.Path != "/healthz" {
		t.Errorf("asked %+v", spec)
	}

	for _, bad := range []any{
		`{"subject":{"kind":"ServiceAccount","name":"x"},"verb":"get","resource":"pods"}`, // no namespace
		`{"subject":{"kind":"User","name":"alice"},"resource":"pods"}`,                    // no verb
		`{"subject":{"kind":"User","name":"alice"},"verb":"get","resource":"pods","extra":1}`,
		`{"subject":{"kind":"User","name":"alice"},"verb":"get","resource":"pods"} {}`,
	} {
		expect(t, e.do(http.MethodPost, "/review", bad), http.StatusBadRequest, plugin.CodeBadRequest)
	}
	if len(*asked) != 2 {
		t.Errorf("invalid reviews reached the cluster: %d", len(*asked))
	}
}

func TestReviewForbidden(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("create", "subjectaccessreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "authorization.k8s.io", Resource: "subjectaccessreviews"}, "", errors.New("RBAC: denied"))
	})
	e := newEnvWith(t, cs)
	rec := e.do(http.MethodPost, "/review", `{"subject":{"kind":"User","name":"alice"},"verb":"get","resource":"pods"}`)
	body := expect(t, rec, http.StatusForbidden, plugin.CodeKubernetes)
	if !strings.Contains(string(body), "needs create on subjectaccessreviews.authorization.k8s.io") {
		t.Errorf("error does not say what is missing: %s", body)
	}
}

func TestReviewRateLimit(t *testing.T) {
	cs := fake.NewClientset()
	answerReviews(cs, func(authv1.SubjectAccessReviewSpec) authv1.SubjectAccessReviewStatus {
		return authv1.SubjectAccessReviewStatus{}
	})
	e := newEnvWith(t, cs)
	review := func() *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/review", `{"subject":{"kind":"User","name":"alice"},"verb":"get","resource":"pods"}`)
	}
	for range reviewsPerWindow {
		expect(t, review(), http.StatusOK, "")
	}
	e.now = e.now.Add(20 * time.Second)
	rec := review()
	expect(t, rec, http.StatusTooManyRequests, plugin.CodeRateLimited)
	if rec.Header().Get("Retry-After") != "40" {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	e.now = e.now.Add(40 * time.Second)
	expect(t, review(), http.StatusOK, "")
}

func TestReviewLimiterPerUser(t *testing.T) {
	now := time.Unix(0, 0)
	l := newReviewLimiter(func() time.Time { return now })
	for range reviewsPerWindow {
		if l.take(1) != 0 {
			t.Fatal("user 1 limited early")
		}
	}
	if l.take(1) == 0 {
		t.Fatal("user 1 not limited")
	}
	if l.take(2) != 0 {
		t.Fatal("user 2 limited by user 1's reviews")
	}
}

// TestReviewNeverImpersonates runs the review through a real client
// against a recording API server: the review must be an ordinary create
// with KubeBench's own credential, never an impersonated request.
func TestReviewNeverImpersonates(t *testing.T) {
	var mu sync.Mutex
	var headers []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		headers = append(headers, r.Header.Clone())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/apis/authorization.k8s.io/v1/subjectaccessreviews" {
			var sar authv1.SubjectAccessReview
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &sar)
			sar.Status.Allowed = true
			_ = json.NewEncoder(w).Encode(sar)
			return
		}
		_, _ = io.WriteString(w, `{"items":[]}`) // every RBAC and ServiceAccount list
	}))
	defer srv.Close()
	cs, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	e := newEnvWith(t, cs)
	res := decode[reviewResponse](t, expect(t, e.do(http.MethodPost, "/review",
		`{"subject":{"kind":"ServiceAccount","namespace":"argocd","name":"argocd-server"},"verb":"delete","resource":"pods","namespace":"argocd"}`),
		http.StatusOK, ""))
	if !res.Allowed || len(res.Grants) != 0 {
		t.Fatalf("review = %+v", res)
	}
	if len(headers) != 6 {
		t.Fatalf("%d requests, want the review and five lists", len(headers))
	}
	for _, h := range headers {
		for name := range h {
			if strings.HasPrefix(strings.ToLower(name), "impersonate-") {
				t.Errorf("request carries %s", name)
			}
		}
	}
}
