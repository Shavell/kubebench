import { create } from 'zustand'
import { useLayoutStore } from '@/core/store/layoutStore'
import { registry } from './registry'

/**
 * Requests to show something in the bottom panel, addressed by kind like
 * editor tabs: `pod-logs` and `pod-exec` with props `{namespace, pod,
 * container}` (design §7). A plugin calls `showInPanel()`; the bottom panel
 * tab that lists the kind in its `kinds` renders the latest request. Like
 * `openTab()`, this keeps cross-plugin flows free of imports between plugins.
 */
export interface PanelRequest {
  kind: string
  props: Record<string, unknown>
  /** Differs for every request, even with equal props (e.g. to start a fresh shell). */
  seq: number
}

const usePanelRequestStore = create<{ requests: Partial<Record<string, PanelRequest>> }>(() => ({
  requests: {},
}))

let seq = 0

/**
 * Shows a `kind` request in the bottom panel tab that handles it and
 * brings that tab to the front. Returns false when no active plugin
 * contributes a panel tab for the kind.
 */
export function showInPanel(kind: string, props: Record<string, unknown>): boolean {
  const tab = registry.bottomPanelTabs().find((t) => t.kinds?.includes(kind))
  if (!tab) return false
  seq += 1
  usePanelRequestStore.setState((s) => ({ requests: { ...s.requests, [kind]: { kind, props, seq } } }))
  useLayoutStore.getState().setActivePanelTab(tab.id)
  return true
}

/** The latest request of `kind`, for the panel tab that renders it. */
export function usePanelRequest(kind: string): PanelRequest | undefined {
  return usePanelRequestStore((s) => s.requests[kind])
}
