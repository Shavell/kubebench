# Security policy

## Supported versions

KubeBench is before 1.0. Security fixes are made for the latest minor
release line only.

| Version | Supported |
|---|---|
| 0.8.x | yes |
| < 0.8 | no (0.1 to 0.4 were never released as tagged versions) |

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

## How the code and the releases are checked

- **Scanning on every push to `main` and every pull request**: CodeQL
  (`security-extended`, Go and TypeScript), govulncheck for the Go server
  and its toolchain, `npm audit` of the runtime npm dependencies, Trivy
  (misconfigurations of the Dockerfile, the Helm chart and the manifests;
  vulnerabilities in `go.mod` and `package-lock.json`) and dependency
  review of pull requests. CodeQL and govulncheck also run weekly, and so
  does a Trivy scan of the latest release image. Findings appear in the
  repository's **Security > Code scanning**.
- **OpenSSF Scorecard** rates the repository's supply-chain practices
  weekly: [scorecard.dev](https://scorecard.dev/viewer/?uri=github.com/Shavell/kubebench).
- **Repository settings**: secret scanning with push protection,
  Dependabot alerts and security updates, and weekly Dependabot version
  updates (npm, Go modules, GitHub Actions pinned by commit SHA, Docker base
  images). `main` is protected and needs the CI checks to pass.
- **Signed releases** (from 0.8.1 on): SPDX SBOMs for every archive and
  package, a keyless cosign signature of `checksums.txt`, a cosign
  signature of the container image, and GitHub build provenance
  attestations for the binaries, packages, chart, manifests, installer,
  offline bundle and image. Verify a download before you install it:

  ```bash
  cosign verify-blob --bundle checksums.txt.sigstore.json \
    --certificate-identity-regexp '^https://github\.com/Shavell/kubebench/\.github/workflows/release\.yml@refs/(heads|tags)/' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    checksums.txt
  sha256sum --check --ignore-missing checksums.txt

  cosign verify ghcr.io/shavell/kubebench:<version> \
    --certificate-identity-regexp '^https://github\.com/Shavell/kubebench/\.github/workflows/release\.yml@refs/(heads|tags)/' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com

  gh attestation verify kubebench_<version>_linux_amd64.tar.gz --repo Shavell/kubebench
  gh attestation verify oci://ghcr.io/shavell/kubebench:<version> --repo Shavell/kubebench
  ```

  [docs/releasing.md](docs/releasing.md#verifying-a-release) has the details.
