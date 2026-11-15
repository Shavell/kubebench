package api

import "net/http"

func (s *Server) handleCrds(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListCrds(r.Context())
	writeResult(w, out, err)
}

func (s *Server) handleCrdInstances(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListCustomResources(r.Context(), r.URL.Query().Get("crd"))
	writeResult(w, out, err)
}
