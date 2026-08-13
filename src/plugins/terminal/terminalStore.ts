import { create } from 'zustand'
import { useLayoutStore } from '@/core/store/layoutStore'

export interface PodTarget {
  namespace: string
  pod: string
  container: string
}

interface TerminalState {
  /** Pod whose logs the bottom-panel Logs tab shows (null = static demo). */
  logsTarget: PodTarget | null
  /** Pod the bottom-panel Terminal tab is exec'd into (null = static demo). */
  execTarget: PodTarget | null
  /** Bumped to force a fresh exec session when re-running Shell on same pod. */
  execNonce: number
  showLogs: (target: PodTarget) => void
  showExec: (target: PodTarget) => void
}

export const useTerminalStore = create<TerminalState>((set) => ({
  logsTarget: null,
  execTarget: null,
  execNonce: 0,
  showLogs: (target) => {
    set({ logsTarget: target })
    useLayoutStore.getState().setActivePanelTab('logs')
  },
  showExec: (target) => {
    set((s) => ({ execTarget: target, execNonce: s.execNonce + 1 }))
    useLayoutStore.getState().setActivePanelTab('terminal')
  },
}))
