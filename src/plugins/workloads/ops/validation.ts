import { tw } from '../i18n'

/**
 * Client-side checks of Kubernetes names, so a dialog explains a typo
 * before the API server would reject it. The server validates again.
 */

const DNS1123_LABEL = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
const DNS1123_SUBDOMAIN = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/
const QUALIFIED_NAME = /^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$/
const LABEL_VALUE = /^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$/

/** A namespace name: a DNS-1123 label. */
export function namespaceNameError(name: string): string | null {
  return name.length <= 63 && DNS1123_LABEL.test(name) ? null : tw('workloadOps.validation.namespaceName')
}

/** A label or taint key: "[prefix/]name", the prefix a DNS subdomain. */
export function qualifiedNameError(key: string): string | null {
  const slash = key.lastIndexOf('/')
  const prefix = slash < 0 ? '' : key.slice(0, slash)
  const name = slash < 0 ? key : key.slice(slash + 1)
  const prefixOk = slash < 0 || (prefix.length <= 253 && DNS1123_SUBDOMAIN.test(prefix))
  return prefixOk && name.length <= 63 && QUALIFIED_NAME.test(name) ? null : tw('workloadOps.validation.qualifiedName')
}

/** A label or taint value (may be empty). */
export function labelValueError(value: string): string | null {
  return value.length <= 63 && LABEL_VALUE.test(value) ? null : tw('workloadOps.validation.labelValue')
}
