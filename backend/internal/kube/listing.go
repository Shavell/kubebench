package kube

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/metadata"

	"kubebench/internal/watch"
)

// listChunk is the page size of the lists KubeBench reads for its views,
// as client-go's pager uses: a large list arrives in several smaller
// responses instead of one that the API server must build at once.
const listChunk = 500

// Truncated says that a list read with a limit left objects out.
type Truncated struct {
	// Remaining is how many objects were left out, or -1 when the cluster
	// did not say (it reports no count for some lists).
	Remaining int64
}

// listPage lists one page of a resource with opts.
type listPage[T any] func(ctx context.Context, opts metav1.ListOptions) ([]T, metav1.ListMeta, error)

// pagedList reads a list for a view (design §4.4, "Lists"): from the API
// server's watch cache (resourceVersion "0": no quorum read from etcd; the
// result may lag the latest write by moments, which live updates correct)
// in pages of listChunk, and with limit > 0 at most limit objects; then
// Truncated says how many were left out. A continue token that expired
// while paging (410 Gone) restarts the list once, unpaged.
func pagedList[T any](ctx context.Context, limit int, page listPage[T]) ([]T, *Truncated, error) {
	var out []T
	opts := metav1.ListOptions{ResourceVersion: "0", Limit: chunkSize(limit, 0)}
	restarted := false
	for {
		items, meta, err := page(ctx, opts)
		if err != nil {
			if opts.Continue != "" && apierrors.IsResourceExpired(err) && !restarted {
				restarted = true
				out = out[:0]
				opts = metav1.ListOptions{ResourceVersion: "0"}
				continue
			}
			return nil, nil, err
		}
		out = append(out, items...)
		if limit > 0 && len(out) >= limit {
			// The cluster may serve more than asked: a watch cache that
			// cannot page answers with the whole list.
			extra := int64(len(out) - limit)
			out = out[:limit]
			switch {
			case meta.Continue == "" && extra == 0:
				return out, nil, nil
			case meta.Continue == "":
				return out, &Truncated{Remaining: extra}, nil
			case meta.RemainingItemCount != nil:
				return out, &Truncated{Remaining: extra + *meta.RemainingItemCount}, nil
			default:
				return out, &Truncated{Remaining: -1}, nil
			}
		}
		if meta.Continue == "" {
			return out, nil, nil
		}
		// A continuation names its snapshot: no resourceVersion with it.
		opts = metav1.ListOptions{Continue: meta.Continue, Limit: chunkSize(limit, len(out))}
	}
}

// chunkSize is the page size for the next page of a list of at most limit
// objects (0 = no limit) of which have are read.
func chunkSize(limit, have int) int64 {
	if limit > 0 && limit-have < listChunk {
		return int64(limit - have)
	}
	return listChunk
}

// CountObjects counts the objects of a live-update topic's resource
// ("pods", "deployments.apps") in namespace ("" = all; ignored for
// cluster-scoped resources). The cache of the hub's informer answers when
// one runs live (Hub.Count); otherwise the objects' metadata is listed
// from the watch cache and counted, stopping at the first page when the
// cluster reports the remaining count. A resource the cluster does not
// serve counts 0, as ListGeneric lists it empty.
func (r *Runtime) CountObjects(ctx context.Context, topic, namespace string) (int, error) {
	if n, ok := r.watch.Count(topic, namespace); ok {
		return n, nil
	}
	resource, group := watch.SplitTopic(topic)
	gvr, namespaced, err := r.ResolveResource(group, resource)
	if meta.IsNoMatchError(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !namespaced {
		namespace = ""
	}
	var res metadata.ResourceInterface = r.meta.Resource(gvr)
	if namespace != "" {
		res = r.meta.Resource(gvr).Namespace(namespace)
	}
	n, err := countPaged(ctx, func(ctx context.Context, opts metav1.ListOptions) (int, metav1.ListMeta, error) {
		l, err := res.List(ctx, opts)
		if err != nil {
			return 0, metav1.ListMeta{}, err
		}
		return len(l.Items), l.ListMeta, nil
	})
	if apierrors.IsNotAcceptable(err) || apierrors.IsUnsupportedMediaType(err) {
		// An aggregated API that cannot list metadata only.
		var dyn dynamic.ResourceInterface = r.Dyn.Resource(gvr)
		if namespace != "" {
			dyn = r.Dyn.Resource(gvr).Namespace(namespace)
		}
		n, err = countPaged(ctx, func(ctx context.Context, opts metav1.ListOptions) (int, metav1.ListMeta, error) {
			l, err := dyn.List(ctx, opts)
			if err != nil {
				return 0, metav1.ListMeta{}, err
			}
			return len(l.Items), metav1.ListMeta{Continue: l.GetContinue(), RemainingItemCount: l.GetRemainingItemCount()}, nil
		})
	}
	if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
		return 0, nil
	}
	return n, err
}

// countPaged counts a list page by page as pagedList reads it.
func countPaged(ctx context.Context, page func(ctx context.Context, opts metav1.ListOptions) (int, metav1.ListMeta, error)) (int, error) {
	opts := metav1.ListOptions{ResourceVersion: "0", Limit: listChunk}
	n := 0
	restarted := false
	for {
		got, lm, err := page(ctx, opts)
		if err != nil {
			if opts.Continue != "" && apierrors.IsResourceExpired(err) && !restarted {
				restarted = true
				n = 0
				opts = metav1.ListOptions{ResourceVersion: "0"}
				continue
			}
			return 0, err
		}
		n += got
		switch {
		case lm.Continue == "":
			return n, nil
		case lm.RemainingItemCount != nil:
			return n + int(*lm.RemainingItemCount), nil
		}
		opts = metav1.ListOptions{Continue: lm.Continue, Limit: listChunk}
	}
}
