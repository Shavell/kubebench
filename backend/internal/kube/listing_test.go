package kube

import (
	"context"
	"strconv"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakePages serves total items in pages as an API server does: Limit and
// Continue (an offset) honoured unless ignoreLimit (a watch cache that
// cannot page); RemainingItemCount only when withCount.
type fakePages struct {
	total       int
	ignoreLimit bool
	withCount   bool
	expireOnce  bool // the first continuation answers 410 Gone
	calls       []metav1.ListOptions
}

func (f *fakePages) page(_ context.Context, opts metav1.ListOptions) ([]int, metav1.ListMeta, error) {
	f.calls = append(f.calls, opts)
	start := 0
	if opts.Continue != "" {
		if f.expireOnce {
			f.expireOnce = false
			return nil, metav1.ListMeta{}, apierrors.NewResourceExpired("continue expired")
		}
		start, _ = strconv.Atoi(opts.Continue)
	}
	end := f.total
	if opts.Limit > 0 && !f.ignoreLimit && start+int(opts.Limit) < f.total {
		end = start + int(opts.Limit)
	}
	items := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		items = append(items, i)
	}
	var meta metav1.ListMeta
	if end < f.total {
		meta.Continue = strconv.Itoa(end)
		if f.withCount {
			n := int64(f.total - end)
			meta.RemainingItemCount = &n
		}
	}
	return items, meta, nil
}

func TestPagedList(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		pages     fakePages
		limit     int
		wantLen   int
		wantCut   int64 // -2 = not truncated
		wantCalls int
	}{
		{"all, paged", fakePages{total: 1234}, 0, 1234, -2, 3},
		{"all, small", fakePages{total: 7}, 0, 7, -2, 1},
		{"limit not reached", fakePages{total: 300}, 1000, 300, -2, 1},
		{"limit exactly reached", fakePages{total: 500}, 500, 500, -2, 1},
		{"limit with count", fakePages{total: 2000, withCount: true}, 700, 700, 1300, 2},
		{"limit without count", fakePages{total: 2000}, 700, 700, -1, 2},
		{"cache ignores limit", fakePages{total: 2000, ignoreLimit: true}, 700, 700, 1300, 1},
		{"expired continue restarts", fakePages{total: 1200, expireOnce: true}, 0, 1200, -2, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.pages
			items, cut, err := pagedList(ctx, tc.limit, f.page)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != tc.wantLen {
				t.Errorf("len = %d, want %d", len(items), tc.wantLen)
			}
			for i, v := range items {
				if v != i {
					t.Fatalf("items[%d] = %d: duplicates or gaps", i, v)
				}
			}
			switch {
			case tc.wantCut == -2 && cut != nil:
				t.Errorf("truncated %+v, want complete", *cut)
			case tc.wantCut != -2 && (cut == nil || cut.Remaining != tc.wantCut):
				t.Errorf("cut = %+v, want remaining %d", cut, tc.wantCut)
			}
			if len(f.calls) != tc.wantCalls {
				t.Errorf("%d calls, want %d: %+v", len(f.calls), tc.wantCalls, f.calls)
			}
			// The first page reads the watch cache; continuations name no version.
			if f.calls[0].ResourceVersion != "0" {
				t.Errorf("first page resourceVersion %q, want 0", f.calls[0].ResourceVersion)
			}
			for _, c := range f.calls[1:] {
				if c.Continue != "" && c.ResourceVersion != "" {
					t.Errorf("continuation with resourceVersion %q", c.ResourceVersion)
				}
			}
			if tc.limit > 0 {
				for _, c := range f.calls {
					if c.Limit > int64(tc.limit) {
						t.Errorf("page limit %d above the list limit %d", c.Limit, tc.limit)
					}
				}
			}
		})
	}
}

func TestCountPaged(t *testing.T) {
	for _, tc := range []struct {
		name      string
		pages     fakePages
		wantCalls int
	}{
		{"one page", fakePages{total: 42}, 1},
		{"paged", fakePages{total: 1700}, 4},
		{"remaining count ends early", fakePages{total: 1700, withCount: true}, 1},
		{"cache ignores limit", fakePages{total: 1700, ignoreLimit: true}, 1},
		{"expired continue restarts", fakePages{total: 1200, expireOnce: true}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.pages
			n, err := countPaged(context.Background(), func(ctx context.Context, opts metav1.ListOptions) (int, metav1.ListMeta, error) {
				items, meta, err := f.page(ctx, opts)
				return len(items), meta, err
			})
			if err != nil || n != f.total {
				t.Fatalf("count = %d, %v; want %d", n, err, f.total)
			}
			if len(f.calls) != tc.wantCalls {
				t.Errorf("%d calls, want %d", len(f.calls), tc.wantCalls)
			}
		})
	}
}
