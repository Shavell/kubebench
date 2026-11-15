import { create } from 'zustand'

/**
 * Bumped by the "focus the list filter" shortcut (`/`): the ListFilterBar
 * of the active tab opens and focuses its search field. Only the active
 * editor tab is rendered, so at most one bar answers.
 */
export const useListFilterFocus = create<{ seq: number }>(() => ({ seq: 0 }))

export function focusListFilter(): void {
  useListFilterFocus.setState((s) => ({ seq: s.seq + 1 }))
}
