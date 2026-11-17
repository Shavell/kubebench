import { create } from 'zustand'
import type { NoteQuery } from '../data/types'

/** The sidebar's filters; kept while the sidebar is switched away. */
export interface NotesFilter {
  search: string
  scope: NonNullable<NoteQuery['scope']>
  allClusters: boolean
  tag: string
}

interface NotesFilterState extends NotesFilter {
  set: (patch: Partial<NotesFilter>) => void
}

export const useNotesFilter = create<NotesFilterState>((set) => ({
  search: '',
  scope: 'all',
  allClusters: false,
  tag: '',
  set: (patch) => set(patch),
}))
