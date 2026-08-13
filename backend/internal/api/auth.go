package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

const sessionTTL = 7 * 24 * time.Hour

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.Store.Authenticate(body.Username, body.Password)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, errors.New("invalid username or password"))
		return
	}
	token, err := s.Store.CreateSession(user.ID, sessionTTL)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "kubebench_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, map[string]any{"username": user.Username, "role": user.Role})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("kubebench_session"); err == nil {
		_ = s.Store.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: "kubebench_session", Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	writeJSON(w, map[string]any{"username": user.Username, "role": user.Role})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	info := map[string]any{
		"user": user.Username,
		"role": user.Role,
	}
	if rt, err := s.runtimeFor(user); err == nil {
		info["context"] = rt.Cluster.Name
		info["server"] = rt.Config.Host
		info["capabilities"] = rt.Caps()
	} else {
		info["context"] = ""
		info["server"] = ""
		info["capabilities"] = map[string]bool{"crds": false, "helm": false, "exec": false}
		info["clusterError"] = err.Error()
	}
	writeJSON(w, info)
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
