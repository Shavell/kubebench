package api

import (
	"net/http"

	"kubebench/internal/kube"
)

func (s *Server) handleCrds(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListCrds(r.Context())
	writeResult(w, out, err)
}

var handleCrdInstances = listHandler(func(r *http.Request, c *call, limit int) ([]kube.CustomResourceInfo, *kube.Truncated, error) {
	return c.rt.ListCustomResources(r.Context(), r.URL.Query().Get("crd"), limit)
})
