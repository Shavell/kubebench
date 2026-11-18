package api

import (
	"context"
	"strconv"
	"time"

	"kubebench/internal/identity/ldapauth"
)

// ldapView shows the directory provider's settings, never its bind
// password, and whether the service account can search.
func ldapView(ctx context.Context, p *ldapauth.Provider) providerView {
	cfg := p.Config()
	secretSet := cfg.BindPassword != ""
	v := providerView{
		Source: p.Source(),
		Name:   p.Name(),
		Settings: []providerSetting{
			{Key: "url", Value: cfg.URL},
			{Key: "directoryType", Value: cfg.Type},
			{Key: "startTls", Value: strconv.FormatBool(cfg.StartTLS)},
			{Key: "caFile", Value: cfg.CAFile},
			{Key: "insecureSkipVerify", Value: strconv.FormatBool(cfg.InsecureSkipVerify)},
			{Key: "bindDn", Value: cfg.BindDN},
			{Key: "bindPassword", SecretSet: &secretSet},
			{Key: "userBaseDn", Value: cfg.UserBaseDN},
			{Key: "userFilter", Value: cfg.UserFilter},
			{Key: "usernameAttribute", Value: cfg.UsernameAttribute},
			{Key: "idAttribute", Value: cfg.IDAttribute},
			{Key: "groupsFrom", Value: cfg.GroupsFrom},
			{Key: "groupBaseDn", Value: cfg.GroupBaseDN},
			{Key: "groupFilter", Value: cfg.GroupFilter},
			{Key: "groupNameAttribute", Value: cfg.GroupNameAttribute},
			{Key: "timeout", Value: cfg.Timeout.String()},
		},
		Warnings: append([]string{}, p.Warnings()...),
	}
	ctx, cancel := context.WithTimeout(ctx, max(providerCheckTimeout, cfg.Timeout+time.Second))
	defer cancel()
	if err := p.Check(ctx); err != nil {
		v.Status.Error = err.Error()
	} else {
		v.Status.OK = true
	}
	return v
}
