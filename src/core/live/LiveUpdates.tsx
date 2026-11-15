import { useEffect } from 'react'
import { useSettingsStore } from '@/core/store/settingsStore'
import { refreshData, useProviders } from '@/data/DataContext'
import { refreshWhere, type QueryScope } from '@/data/invalidation'
import { startLiveUpdates } from './liveController'
import { setLiveStatus, useLiveStore, type LiveState } from './liveStore'

/** How often topics the server cannot watch are refreshed when auto-refresh is off. */
const FALLBACK_POLL_SECONDS = 30

interface PollPlan {
  seconds: number
  run: () => void
}

/**
 * The timer refreshes that complement (or replace) live updates; the
 * auto-refresh setting stays the fallback:
 * - live: queries of polled topics, and queries without topics when
 *   auto-refresh is on;
 * - polling: everything, at the auto-refresh interval or every 30 s;
 * - connecting/offline: everything, when auto-refresh is on;
 * - off: nothing.
 */
function pollPlan(state: LiveState, polled: ReadonlySet<string>, autoRefreshSeconds: number): PollPlan | null {
  const polledScope = (scope: QueryScope) => Boolean(scope.topics?.some((t) => polled.has(t)))
  switch (state) {
    case 'live':
      if (autoRefreshSeconds > 0) {
        return { seconds: autoRefreshSeconds, run: () => refreshWhere((s) => !s.topics?.length || polledScope(s)) }
      }
      return polled.size ? { seconds: FALLBACK_POLL_SECONDS, run: () => refreshWhere(polledScope) } : null
    case 'polling':
      return { seconds: autoRefreshSeconds || FALLBACK_POLL_SECONDS, run: refreshData }
    case 'connecting':
    case 'offline':
      return autoRefreshSeconds > 0 ? { seconds: autoRefreshSeconds, run: refreshData } : null
    case 'off':
      return null
  }
}

/**
 * Runs live updates while the setting is on (design §7.10), and the timer
 * refreshes of pollPlan. Mounted once by the shell; renders nothing.
 */
export function LiveUpdates() {
  const { live } = useProviders()
  const enabled = useSettingsStore((s) => s.liveUpdates)
  const autoRefreshSeconds = useSettingsStore((s) => s.autoRefreshSeconds)
  const state = useLiveStore((s) => s.state)
  const polledKey = useLiveStore((s) => s.polled.map((p) => p.topic).join(','))

  useEffect(() => {
    if (enabled) return startLiveUpdates(live)
  }, [enabled, live])

  useEffect(() => {
    if (!enabled) {
      setLiveStatus({ state: autoRefreshSeconds > 0 ? 'polling' : 'off', reason: null, polled: [], watched: [] })
    }
  }, [enabled, autoRefreshSeconds])

  useEffect(() => {
    const plan = pollPlan(state, new Set(polledKey ? polledKey.split(',') : []), autoRefreshSeconds)
    if (!plan) return
    const timer = setInterval(plan.run, plan.seconds * 1000)
    return () => clearInterval(timer)
  }, [state, polledKey, autoRefreshSeconds])

  return null
}
