/**
 * A Kubernetes 403: the cluster's credential lacks the RBAC for a request
 * (design §7.16). The server explains it with a `kubernetes.forbidden*`
 * reason code (backend/internal/plugin/forbidden.go).
 */
export function isKubernetesForbidden(reason: string | null | undefined): boolean {
  return reason === 'kubernetes.forbidden' || Boolean(reason?.startsWith('kubernetes.forbidden_'))
}
