import { create } from 'zustand'
import type { BuildOptions } from '../model/types'

interface OptionsState {
  options: BuildOptions
  setOption: (name: keyof BuildOptions, value: boolean) => void
}

/** Display options, shared by every topology tab of the session. */
export const useTopologyOptions = create<OptionsState>((set) => ({
  options: { showConfig: true, showInactiveReplicaSets: false },
  setOption: (name, value) => set((s) => ({ options: { ...s.options, [name]: value } })),
}))
