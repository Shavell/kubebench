import { dataMode, type DataMode } from '@/data/mode'

/**
 * Whose browser-kept preferences these are: the signed-in user (demo mode
 * has one shared demo user). Keys the per-user stores (aliases, display).
 */
export function prefsOwner(mode: DataMode = dataMode): string {
  return `${mode.mode}:${mode.user ?? ''}`
}
