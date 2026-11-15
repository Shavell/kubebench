package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]{0,63}$`)

type assignmentView struct {
	ClusterID int64  `json:"clusterId"`
	Cluster   string `json:"cluster"`
	Role      string `json:"role"`
}

type userView struct {
	ID                 int64            `json:"id"`
	Username           string           `json:"username"`
	Role               string           `json:"role"`
	MustChangePassword bool             `json:"mustChangePassword"`
	Clusters           []string         `json:"clusters"` // assigned cluster names, for v0.4 clients
	Assignments        []assignmentView `json:"assignments"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, _ *http.Request, _ *call) {
	users, err := s.store.ListUsers()
	if err != nil {
		writeErr(w, err)
		return
	}
	assignments, err := s.store.Assignments()
	if err != nil {
		writeErr(w, err)
		return
	}
	byUser := map[int64][]store.Assignment{}
	for _, a := range assignments {
		byUser[a.UserID] = append(byUser[a.UserID], a)
	}
	out := make([]userView, 0, len(users))
	for _, u := range users {
		v := userView{
			ID: u.ID, Username: u.Username, Role: u.Role, MustChangePassword: u.MustChangePassword,
			Clusters: []string{}, Assignments: []assignmentView{},
		}
		for _, a := range byUser[u.ID] {
			v.Clusters = append(v.Clusters, a.ClusterName)
			v.Assignments = append(v.Assignments, assignmentView{ClusterID: a.ClusterID, Cluster: a.ClusterName, Role: a.Role})
		}
		out = append(out, v)
	}
	plugin.WriteJSON(w, out)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, c *call) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Role == "" {
		body.Role = store.RoleUser
	}
	c.setAudit("user/"+body.Username, auditJSON(map[string]string{"role": body.Role}))
	if !usernamePattern.MatchString(body.Username) {
		writeErr(w, badRequest("a username is 1-64 letters, digits, '.', '_', '@' or '-'"))
		return
	}
	if err := validateGlobalRole(body.Role); err != nil {
		writeErr(w, err)
		return
	}
	if err := validatePassword(body.Password); err != nil {
		writeErr(w, err)
		return
	}
	// The admin chose this password, so the user replaces it at first sign-in.
	id, err := s.store.CreateUser(body.Username, body.Password, body.Role, true)
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]int64{"id": id})
}

// handleUpdateUser changes a user's global role and/or resets their
// password. A reset password is known to the admin, so the user must
// replace it and their sessions end (except the caller's own).
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request, c *call) {
	user, err := s.pathUser(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Role     *string `json:"role"`
		Password *string `json:"password"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	detail := map[string]any{}
	if body.Role != nil {
		detail["role"] = *body.Role
	}
	if body.Password != nil {
		detail["passwordReset"] = true // never the password itself
	}
	c.setAudit("user/"+user.Username, auditJSON(detail))
	if len(detail) == 0 {
		writeErr(w, badRequest("nothing to update: send role and/or password"))
		return
	}
	if body.Role != nil {
		if err := validateGlobalRole(*body.Role); err != nil {
			writeErr(w, err)
			return
		}
	}
	if body.Password != nil {
		if err := validatePassword(*body.Password); err != nil {
			writeErr(w, err)
			return
		}
	}
	if body.Role != nil {
		if err := s.store.SetUserRole(user.ID, *body.Role); err != nil {
			writeErr(w, lastAdminConflict(err, "demote"))
			return
		}
	}
	if body.Password != nil {
		keep := ""
		if user.ID == c.user.ID {
			keep = c.token
		}
		if err := s.store.SetPassword(user.ID, *body.Password, true, keep); err != nil {
			writeErr(w, err)
			return
		}
	}
	writeOK(w)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request, c *call) {
	user, err := s.pathUser(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	c.setAudit("user/"+user.Username, "")
	if user.ID == c.user.ID {
		writeErr(w, errorf(http.StatusConflict, plugin.CodeConflict, "you cannot delete your own account"))
		return
	}
	if err := s.store.DeleteUser(user.ID); err != nil {
		writeErr(w, lastAdminConflict(err, "delete"))
		return
	}
	writeOK(w)
}

// handleSetAssignments replaces a user's cluster assignments. It accepts
// {"assignments":[{"clusterId":1,"role":"viewer"}]} or the v0.4 shape
// {"clusterIds":[1,2]}, which grants operator (v0.4's full access).
func (s *Server) handleSetAssignments(w http.ResponseWriter, r *http.Request, c *call) {
	user, err := s.pathUser(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Assignments []struct {
			ClusterID int64  `json:"clusterId"`
			Role      string `json:"role"`
		} `json:"assignments"`
		ClusterIDs []int64 `json:"clusterIds"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	var list []store.Assignment
	switch {
	case body.Assignments != nil:
		for _, a := range body.Assignments {
			list = append(list, store.Assignment{ClusterID: a.ClusterID, Role: a.Role})
		}
	case body.ClusterIDs != nil:
		for _, id := range body.ClusterIDs {
			list = append(list, store.Assignment{ClusterID: id, Role: store.ClusterRoleOperator})
		}
	default:
		writeErr(w, badRequest("send assignments (or the v0.4 clusterIds)"))
		return
	}
	granted := make([]string, 0, len(list))
	for _, a := range list {
		granted = append(granted, fmt.Sprintf("%d:%s", a.ClusterID, a.Role))
	}
	c.setAudit("user/"+user.Username, "")
	c.setAuditItems(map[string]any{}, "assignments", granted)
	for _, a := range list {
		if !store.ValidClusterRole(a.Role) {
			writeErr(w, badRequest("invalid cluster role %q: use viewer or operator", a.Role))
			return
		}
	}
	if err := s.store.SetAssignments(user.ID, list); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			err = errorf(http.StatusNotFound, plugin.CodeNotFound, "%v", err)
		}
		writeErr(w, err)
		return
	}
	writeOK(w)
}

// pathUser loads the user named by the {id} path value.
func (s *Server) pathUser(r *http.Request) (*store.User, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, badRequest("invalid user id %q", r.PathValue("id"))
	}
	user, err := s.store.GetUser(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errorf(http.StatusNotFound, plugin.CodeNotFound, "user %d not found", id)
	}
	return user, err
}

func validateGlobalRole(role string) error {
	if role != store.RoleAdmin && role != store.RoleUser {
		return badRequest("invalid role %q: use admin or user", role)
	}
	return nil
}

// lastAdminConflict explains the store's last-admin protection.
func lastAdminConflict(err error, verb string) error {
	if errors.Is(err, store.ErrLastAdmin) {
		return errorf(http.StatusConflict, plugin.CodeConflict, "cannot %s the last admin: promote another user first", verb)
	}
	return err
}

// auditJSON encodes a short audit detail.
func auditJSON(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false) // keep commands readable: "a > b", not "a \u003e b"
	if err := enc.Encode(v); err != nil {
		return ""
	}
	return strings.TrimSuffix(b.String(), "\n")
}
