package kube

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"

	"kubebench/internal/reason"
)

// RequestError is a problem with what the caller asked for, found before
// anything was sent to the cluster (the API reports it as 400 bad_request).
type RequestError struct {
	Code   reason.Code
	Params reason.Params
}

func (e *RequestError) Error() string { return e.Code.Text(e.Params) }

// Reason implements reason.Reasoned.
func (e *RequestError) Reason() (reason.Code, reason.Params) { return e.Code, e.Params }

// Object is one object of any kind, mapped to its REST resource by
// ResolveObject.
type Object struct {
	Resource   schema.GroupVersionResource
	Kind       string
	Namespaced bool
	// Namespace is empty for cluster-scoped kinds.
	Namespace string
	Name      string
}

// Target is the object's audit target, "<kind>/<namespace>/<name>" with
// the kind in lower case, like the targets the API derives from queries.
func (o Object) Target() string {
	kind := strings.ToLower(o.Kind)
	if o.Namespace == "" {
		return kind + "/" + o.Name
	}
	return kind + "/" + o.Namespace + "/" + o.Name
}

// ResolveObject maps apiVersion ("apps/v1", "v1") and kind to the
// object's resource through the RESTMapper. A namespaced kind needs a
// namespace (a *RequestError otherwise); a cluster-scoped kind ignores it.
func (r *Runtime) ResolveObject(apiVersion, kind, namespace, name string) (Object, error) {
	gvr, namespaced, err := r.ResolveKind(apiVersion, kind)
	if err != nil {
		return Object{}, err
	}
	obj := Object{Resource: gvr, Kind: kind, Namespaced: namespaced, Namespace: namespace, Name: name}
	switch {
	case !namespaced:
		obj.Namespace = ""
	case namespace == "":
		return Object{}, &RequestError{Code: reasonNamespacedKind, Params: reason.Params{"kind": kind}}
	}
	return obj, nil
}

// client is the dynamic client for the object's resource and namespace.
func (r *Runtime) client(o Object) dynamic.ResourceInterface {
	if o.Namespaced {
		return r.Dyn.Resource(o.Resource).Namespace(o.Namespace)
	}
	return r.Dyn.Resource(o.Resource)
}

// isSecrets reports whether gvr is the core Secrets resource, whose
// values KubeBench only shows through RevealSecret.
func isSecrets(gvr schema.GroupVersionResource) bool {
	return gvr.Group == "" && gvr.Resource == "secrets"
}

// getObject fetches the object as unstructured content, Secret values and
// credentials redacted (redactObject).
func (r *Runtime) getObject(ctx context.Context, o Object) (map[string]any, error) {
	obj, err := r.client(o).Get(ctx, o.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	redactObject(o.Resource.GroupResource(), obj.Object)
	return obj.Object, nil
}

// ObjectRef names an object for ObjectYAML. With APIVersion it may be of
// any kind the cluster serves; without, Kind is one of the kinds v0.4's
// YAML views knew by name (Pod, Deployment, Service, Node, Namespace, the
// generic catalog's kinds, CRD kinds).
type ObjectRef struct {
	APIVersion, Kind, Namespace, Name string
}

// ObjectYAML renders one object as YAML without managedFields; neat also
// drops what Neat strips. Secret values and credentials are always
// redacted.
func (r *Runtime) ObjectYAML(ctx context.Context, ref ObjectRef, neat bool) (string, error) {
	var obj map[string]any
	var err error
	if ref.APIVersion != "" {
		var o Object
		if o, err = r.ResolveObject(ref.APIVersion, ref.Kind, ref.Namespace, ref.Name); err != nil {
			return "", err
		}
		obj, err = r.getObject(ctx, o)
	} else {
		obj, err = r.objectByKind(ctx, ref.Kind, ref.Namespace, ref.Name)
	}
	if err != nil {
		return "", err
	}
	return renderYAML(obj, neat)
}

// renderYAML marshals obj without managedFields, neat on request.
func renderYAML(obj map[string]any, neat bool) (string, error) {
	unstructured.RemoveNestedField(obj, "metadata", "managedFields")
	if neat {
		obj = Neat(obj)
	}
	out, err := yaml.Marshal(obj)
	return string(out), err
}
