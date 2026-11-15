import { create } from 'zustand'
import { DEFAULT_FILTER, type FindingsFilter, type Grouping } from './model/filters'

/**
 * Page state shared by the plugin's tabs: the scan options and whether a
 * scan runs (any page may start one), and the findings filter, which the
 * overview, the catalog and resource actions set before opening the
 * findings tab (an open tab keeps its props, so they cannot carry it).
 */
interface SecurityState {
  includeSystem: boolean
  scanning: boolean
  filter: FindingsFilter
  grouping: Grouping
  setIncludeSystem: (value: boolean) => void
  setScanning: (value: boolean) => void
  setFilter: (patch: Partial<FindingsFilter>) => void
  /** Replaces the filter: the defaults plus `patch`. */
  resetFilter: (patch?: Partial<FindingsFilter>) => void
  setGrouping: (grouping: Grouping) => void
}

export const useSecurityStore = create<SecurityState>((set) => ({
  includeSystem: false,
  scanning: false,
  filter: DEFAULT_FILTER,
  grouping: 'none',
  setIncludeSystem: (includeSystem) => set({ includeSystem }),
  setScanning: (scanning) => set({ scanning }),
  setFilter: (patch) => set((s) => ({ filter: { ...s.filter, ...patch } })),
  resetFilter: (patch) => set({ filter: { ...DEFAULT_FILTER, ...patch } }),
  setGrouping: (grouping) => set({ grouping }),
}))
