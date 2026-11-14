import { useCallback, useEffect, useRef, useState } from 'react'
import { useProviders } from '@/data/DataContext'
import type { AuditEntry, AuditQuery } from '@/data/types'

/** Filters of GET /api/audit: user and cluster match exactly, action is a substring. */
export interface AuditFilters {
  user: string
  cluster: string
  action: string
}

interface AuditLogState {
  items: AuditEntry[]
  /** `before` of the next (older) page; null once everything is loaded. */
  nextBefore: number | null
  loading: boolean
  /** The last request's failure; earlier pages stay listed when "load more" fails. */
  error: unknown
}

const PAGE_SIZE = 100

const toQuery = (filters: AuditFilters): AuditQuery => ({
  user: filters.user || undefined,
  cluster: filters.cluster || undefined,
  action: filters.action || undefined,
})

/**
 * The audit log, newest first, one page at a time (design §3.4). Changing
 * the filters or reloading starts again at the newest entry; `loadMore`
 * appends the next page. It deliberately ignores the global refresh bus,
 * which would throw away the pages already loaded.
 */
export function useAuditLog(filters: AuditFilters) {
  const { audit } = useProviders()
  const [state, setState] = useState<AuditLogState>({ items: [], nextBefore: null, loading: true, error: null })
  const [reloads, setReloads] = useState(0)
  // Only the latest request may update the state.
  const latest = useRef(0)

  const { user, cluster, action } = filters
  useEffect(() => {
    const request = ++latest.current
    setState((s) => ({ ...s, loading: true, error: null }))
    audit.listAudit({ ...toQuery({ user, cluster, action }), limit: PAGE_SIZE }).then(
      (page) => {
        if (request === latest.current) setState({ ...page, loading: false, error: null })
      },
      (error: unknown) => {
        if (request === latest.current) setState({ items: [], nextBefore: null, loading: false, error })
      },
    )
  }, [audit, user, cluster, action, reloads])

  const loadMore = useCallback(() => {
    if (state.nextBefore === null || state.loading) return
    const request = ++latest.current
    setState((s) => ({ ...s, loading: true, error: null }))
    audit.listAudit({ ...toQuery({ user, cluster, action }), limit: PAGE_SIZE, before: state.nextBefore }).then(
      (page) => {
        if (request === latest.current) {
          setState((s) => ({ items: [...s.items, ...page.items], nextBefore: page.nextBefore, loading: false, error: null }))
        }
      },
      (error: unknown) => {
        if (request === latest.current) setState((s) => ({ ...s, loading: false, error }))
      },
    )
  }, [audit, user, cluster, action, state.nextBefore, state.loading])

  const reload = useCallback(() => setReloads((n) => n + 1), [])

  return { ...state, loadMore, reload }
}
