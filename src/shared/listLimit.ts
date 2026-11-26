import { useState } from 'react'

/** How many rows a list view asks for at first, and how many more each "Load more" adds. */
export const LIST_PAGE = 2_000

/**
 * The `limit` of a list view (design §4.4, "Lists"): LIST_PAGE rows, raised
 * by LIST_PAGE with each loadMore(); back to LIST_PAGE whenever `resetKey`
 * (what the list shows: its resource, namespace, …) changes.
 */
export function useListLimit(resetKey: string): { limit: number; loadMore: () => void } {
  const [state, setState] = useState({ key: resetKey, limit: LIST_PAGE })
  const limit = state.key === resetKey ? state.limit : LIST_PAGE
  return { limit, loadMore: () => setState({ key: resetKey, limit: limit + LIST_PAGE }) }
}
