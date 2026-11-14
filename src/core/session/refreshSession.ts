import { dataMode, dataModeFromInfo, setDataMode } from '@/data/mode'
import type { SessionProvider } from '@/data/providers'

/**
 * Re-reads the session facts (GET /api/info) after a change to the active
 * cluster that does not switch clusters — a rename, a new environment or
 * read-only flag — so the status bar and permission gates follow without
 * a reload.
 */
export async function refreshSession(session: SessionProvider): Promise<void> {
  setDataMode(dataModeFromInfo(dataMode.mode, await session.info()))
}
