package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"kubebench/internal/store"
)

type userView struct {
	store.User
	Clusters []string `json:"clusters"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, _ *http.Request) {
	users, err := s.Store.ListUsers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := []userView{}
	for _, u := range users {
		ids, _ := s.Store.UserClusterIDs(u.ID)
		names := []string{}
		for _, id := range ids {
			if c, err := s.Store.GetCluster(id); err == nil {
				names = append(names, c.Name)
			}
		}
		out = append(out, userView{User: u, Clusters: names})
	}
	writeJSON(w, out)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if body.Username == "" || len(body.Password) < 4 {
		writeErr(w, http.StatusBadRequest, errors.New("username and a password of at least 4 characters are required"))
		return
	}
	if body.Role != "admin" {
		body.Role = "user"
	}
	id, err := s.Store.CreateUser(body.Username, body.Password, body.Role)
	if err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, map[string]int64{"id": id})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if id == currentUser(r).ID {
		writeErr(w, http.StatusBadRequest, errors.New("cannot delete your own account"))
		return
	}
	if err := s.Store.DeleteUser(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleAssignClusters(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var body struct {
		ClusterIDs []int64 `json:"clusterIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.AssignClusters(id, body.ClusterIDs); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
