import type { ArgoDestination, ArgoSource } from './data/types'

/** Display helpers for Argo CD values that are not UI strings (URLs, revisions). */

/** Commit SHAs shortened like Git does; tags and branches stay as they are. */
export function shortRevision(revision: string): string {
  return /^[0-9a-f]{40}$/.test(revision) ? revision.slice(0, 7) : revision
}

/** "repo › path @ revision" or "repo › chart @ version" of a source. */
export function sourceText(source: Pick<ArgoSource, 'path' | 'chart' | 'targetRevision'> & { repo?: string; repoURL?: string }): string {
  const repo = (source.repoURL ?? source.repo ?? '').replace(/^https?:\/\//, '').replace(/\.git$/, '')
  const where = source.chart || source.path
  const revision = source.targetRevision ? ` @ ${source.targetRevision}` : ''
  return `${repo}${where ? ` › ${where}` : ''}${revision}`
}

/** "namespace · cluster" of a destination; in-cluster URLs read "in-cluster" as in Argo CD. */
export function destinationText(destination: ArgoDestination): string {
  const cluster =
    destination.name || (destination.server === 'https://kubernetes.default.svc' ? 'in-cluster' : destination.server)
  return [destination.namespace, cluster].filter(Boolean).join(' · ')
}
