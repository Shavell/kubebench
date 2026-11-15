import { refreshData } from '@/data/DataContext'
import { notify } from '@/shared/notify'

/**
 * Runs an operation that needs no dialog (cordon, pause, restart, …):
 * a success toast (the message, or what `success` does with the result),
 * or an error toast with the reason; then a refresh either way, since a
 * failed request may still have changed something.
 */
export async function runOp<T>(operation: () => Promise<T>, success: string | ((result: T) => void)): Promise<void> {
  try {
    const result = await operation()
    if (typeof success === 'string') notify.success(success)
    else success(result)
  } catch (err) {
    notify.error(err)
  } finally {
    refreshData()
  }
}
