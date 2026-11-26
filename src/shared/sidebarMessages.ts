import { isKubernetesForbidden } from './forbidden.ts'

/** A failed query as SidebarErrors takes it: its message and, when known, its reason code. */
export interface SidebarError {
  error: string | null
  errorReason?: string | null
}

/**
 * The messages SidebarErrors shows (pure): each distinct message once,
 * except Kubernetes 403s (design §7.16), which a namespace-scoped
 * credential gets for every cluster-wide count: they are summed up as one
 * quiet notice (`forbidden`) instead.
 */
export function sidebarMessages(errors: readonly (string | SidebarError | null | undefined)[]): {
  messages: string[]
  forbidden: boolean
} {
  let forbidden = false
  const messages = new Set<string>()
  for (const item of errors) {
    if (!item) continue
    if (typeof item === 'string') {
      messages.add(item)
    } else if (isKubernetesForbidden(item.errorReason)) {
      forbidden = true
    } else if (item.error) {
      messages.add(item.error)
    }
  }
  return { messages: [...messages], forbidden }
}
