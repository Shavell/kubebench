# Security policy

## Supported versions

KubeBench is before 1.0. Security fixes are made for the latest minor
release line only.

| Version | Supported |
|---|---|
| 0.5.x | yes |
| < 0.5 | no (never released as tagged versions) |

## Reporting a vulnerability

Please **do not open a public issue** for a security problem.

Report it privately through GitHub's private vulnerability reporting:
[github.com/Shavell/kubebench/security/advisories/new](https://github.com/Shavell/kubebench/security/advisories/new)
(repository **Security** tab → **Report a vulnerability**).

Include what you can of:

- the KubeBench version (`kubebench --version` or Help > About) and how it is
  installed (Helm chart, manifests, package, Docker, binary);
- the affected component (API route, plugin, UI view, chart, installer);
- steps to reproduce, or a proof of concept against a test cluster;
- the impact you see (for example: a viewer can change objects, a Secret
  value leaks, a pasted kubeconfig runs a command).

We acknowledge reports within a few working days, keep you informed while
we investigate, and credit you in the release notes unless you prefer
otherwise. Please give us reasonable time to release a fix before you
disclose the issue.

## Security model in short

- **Credentials stay on the server.** KubeBench stores cluster credentials
  (in-cluster ServiceAccount, kubeconfigs, optional cloud credentials) in its
  SQLite database (mode `0600`) and never sends them to the browser.
- **Roles.** Global `admin` / `user`; per assigned cluster `viewer` (read)
  or `operator` (read, write, exec). Every API route declares its access
  level and the server enforces it before anything reaches the cluster;
  the cluster's own RBAC still applies on top.
- **Read-only clusters** refuse every change and shell for everyone,
  admins included.
- **Audit log** of every change, shell and admin action, refused attempts
  included; request bodies are never recorded.
- **Secret redaction** on every read path (including Helm manifests and the
  `last-applied-configuration` annotation); revealing values is a separate,
  audited permission (exec).
- **Kubeconfig safety.** Pasted kubeconfigs may not run exec plugins, auth
  providers or read server files unless `KUBEBENCH_ALLOW_KUBECONFIG_EXEC=true`;
  adding clusters by server path is controlled by
  `KUBEBENCH_ALLOW_PATH_KUBECONFIG` (off inside a cluster).
- **Cloud identity.** EKS, GKE and AKS tokens are obtained in process.
  Pasted kubeconfigs need per-cluster credentials unless
  `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY=true`, which lets every admin use
  (and obtain the token of) the server's own cloud identity; grant that
  identity only what KubeBench should have.
- **Sessions.** HttpOnly SameSite cookies, `Secure` on TLS, login throttling,
  Origin checks on state-changing requests, forced password changes for
  passwords someone else knows.

The full threat model and a hardening checklist for operators are in
[docs/security.md](docs/security.md).
