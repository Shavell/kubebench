import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Alias } from '@/core/palette/kubectl'
import { useDataMode } from '@/data/mode'
import { prefsOwner } from './prefsOwner'

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

/** The signed-in user's aliases and a setter for them. */
export function useAliases(): { aliases: Alias[]; setAliases: (aliases: Alias[]) => void } {
  const owner = prefsOwner(useDataMode())
  const aliases = useAliasStore((s) => s.byUser[owner]) ?? NONE
  const setAliases = useAliasStore((s) => s.setAliases)
  return { aliases, setAliases: (next) => setAliases(owner, next) }
}
