// Relative import with extension: unit-tested with node --test.
import type { ApplyResult } from '../../data/types.ts'

/** The reason of an edit refused because the object changed on the server since editing started. */
export const CHANGED_ON_SERVER = 'edit.changed_on_server'

/** Whether r is an edit refused because the object changed on the server (a stale resourceVersion). */
export const isChangedOnServer = (r: ApplyResult) => r.code === 'conflict' && r.reason === CHANGED_ON_SERVER

/** Whether r is a server-side apply conflict with another field manager, which a forced apply takes over. */
export const isFieldConflict = (r: ApplyResult) => r.code === 'conflict' && !isChangedOnServer(r)
