import { useState } from 'react'

/** Whether an action of this control is running, and a runner that tracks it. */
export function useBusy(): [boolean, (action: () => Promise<void>) => void] {
  const [busy, setBusy] = useState(false)
  const run = (action: () => Promise<void>) => {
    setBusy(true)
    void action().finally(() => setBusy(false))
  }
  return [busy, run]
}
