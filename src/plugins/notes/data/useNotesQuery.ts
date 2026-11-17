import type { DependencyList } from 'react'
import { usePluginQuery } from '@/data/pluginData'
import type { QueryState } from '@/data/DataContext'
import { useFocusRevision } from '../model/focusRevision'
import type { NotesProvider } from './provider'
import { notesData } from '.'

/**
 * A query over the notes provider. It re-runs on refreshData() (after the
 * UI's own changes), on auto-refresh, and when the window regains focus
 * (see focusRevision): notes have no live-update topic.
 */
export function useNotesQuery<T>(query: (notes: NotesProvider) => Promise<T>, deps: DependencyList): QueryState<T> {
  const focus = useFocusRevision()
  return usePluginQuery(() => query(notesData()), [...deps, focus])
}
