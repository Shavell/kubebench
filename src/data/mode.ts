/**
 * Which data source the app was bootstrapped with. Set once in main.tsx
 * before render; components read it for display purposes only.
 */
export interface DataMode {
  mode: 'live' | 'mock'
  context: string
  server?: string
}

export const dataMode: DataMode = { mode: 'mock', context: 'kind-dev-cluster (mock)' }

export function setDataMode(next: DataMode): void {
  Object.assign(dataMode, next)
}
