import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Alias } from '@/core/palette/kubectl'
import { dataMode, useDataMode, type DataMode } from '@/data/mode'

/**
 * The palette's command aliases (`kgp` → `get pods`), kept in this browser
 * for each signed-in user, like the other settings. Edited in Settings.
 */
interface AliasState {
  byUser: Record<string, Alias[]>
  setAliases: (user: string, aliases: Alias[]) => void
}

export const useAliasStore = create<AliasState>()(
  persist(
    (set) => ({
      byUser: {},
      setAliases: (user, aliases) => set((s) => ({ byUser: { ...s.byUser, [user]: aliases } })),
    }),
    { name: 'kubebench.aliases' },
  ),
)

const NONE: Alias[] = []

/** Whose aliases these are: the signed-in user (demo mode has one shared demo user). */
export function aliasOwner(mode: DataMode = dataMode): string {
  return `${mode.mode}:${mode.user ?? ''}`
}

/** The signed-in user's aliases and a setter for them. */
export function useAliases(): { aliases: Alias[]; setAliases: (aliases: Alias[]) => void } {
  const owner = aliasOwner(useDataMode())
  const aliases = useAliasStore((s) => s.byUser[owner]) ?? NONE
  const setAliases = useAliasStore((s) => s.setAliases)
  return { aliases, setAliases: (next) => setAliases(owner, next) }
}
