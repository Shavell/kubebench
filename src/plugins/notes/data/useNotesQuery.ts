import type { DependencyList } from 'react'
import { usePluginQuery } from '@/data/pluginData'
import type { QueryState } from '@/data/DataContext'
import type { NotesProvider } from './provider'
import { notesData } from '.'

/**
 * The live-update topic of notes (design §7.13): the server reports a
 * change to a shared note to every user of its cluster, and a change to a
 * personal note to its owner only.
 */
export const NOTES_TOPIC = 'kubebench:notes'

/**
 * A query over the notes provider. It re-runs on refreshData() (after the
 * UI's own changes), when the live-update stream reports a note change
 * (another user's, or this user's in another tab) and on auto-refresh.
 */
export function useNotesQuery<T>(query: (notes: NotesProvider) => Promise<T>, deps: DependencyList): QueryState<T> {
  return usePluginQuery(() => query(notesData()), deps, { topics: [NOTES_TOPIC] })
}
