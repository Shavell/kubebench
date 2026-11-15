# Backend API

The backend is the Go binary in [`backend/`](../backend) (`npm run server:go`
in development). Its HTTP API is specified in
[`design-v0.5.md`](design-v0.5.md): access levels, errors and audit in §3,
the core endpoints in §4, the backend plugin contract in §5 and each
plugin's routes (under `/api/plugins/<id>/`) in §7. Every route is
registered once, with its access level, in
[`backend/internal/api/routes.go`](../backend/internal/api/routes.go);
plugin routes in each plugin's `Routes` method under
`backend/internal/plugins/`. Package layout, database schema and the
cluster runtime are described in [`design-go-backend.md`](design-go-backend.md);
the RBAC the in-cluster deployment needs in [`deployment.md`](deployment.md).
