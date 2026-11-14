import { create } from 'zustand'
import { describeError } from './apiErrorText'

export type ToastKind = 'success' | 'error' | 'info'

export interface ToastOptions {
  /** Second line, e.g. the server's error message. */
  detail?: string
  /**
   * Milliseconds until the toast closes itself; 0 keeps it until dismissed.
   * Defaults: 5 s for success and info, 10 s for errors. Hovering pauses it.
   */
  timeout?: number
  /** One button on the toast, e.g. "Open" or "Undo"; the toast closes after it runs. */
  action?: { label: string; run: () => void }
}

export interface Toast extends ToastOptions {
  id: number
  kind: ToastKind
  message: string
  timeout: number
}

const DEFAULT_TIMEOUT: Record<ToastKind, number> = { success: 5000, info: 5000, error: 10000 }
/** Older toasts are dropped beyond this many. */
const MAX_TOASTS = 5

interface ToastState {
  toasts: Toast[]
  push: (toast: Omit<Toast, 'id'>) => number
  dismiss: (id: number) => void
}

let nextId = 1

export const useToastStore = create<ToastState>((set) => ({
  toasts: [],
  push: (toast) => {
    const id = nextId++
    set((s) => ({ toasts: [...s.toasts, { ...toast, id }].slice(-MAX_TOASTS) }))
    return id
  },
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}))

function show(kind: ToastKind, message: string, options: ToastOptions = {}): number {
  return useToastStore.getState().push({
    ...options,
    kind,
    message,
    timeout: options.timeout ?? DEFAULT_TIMEOUT[kind],
  })
}

/**
 * Transient notifications (design §6.6), rendered by <Toaster/>. Messages
 * are already translated. Each call returns the toast id for
 * `notify.dismiss(id)`.
 *
 *   notify.success(t('pods.deleted', { name }))
 *   notify.error(err)                                    // friendly text for ApiError codes
 *   notify.error(t('pods.deleteFailed', { name }), { detail: errorMessage(err) })
 */
export const notify = {
  success: (message: string, options?: ToastOptions) => show('success', message, options),
  info: (message: string, options?: ToastOptions) => show('info', message, options),
  /** Takes a translated message or a thrown value (its ApiError code picks the wording). */
  error: (messageOrError: unknown, options?: ToastOptions) => {
    if (typeof messageOrError === 'string') return show('error', messageOrError, options)
    const { message, detail } = describeError(messageOrError)
    return show('error', message, { detail, ...options })
  },
  dismiss: (id: number) => useToastStore.getState().dismiss(id),
}
