import { create } from 'zustand'

/** Bumped to ask the status bar's namespace item to open its picker. */
export const useNamespacePickerRequest = create<{ seq: number }>(() => ({ seq: 0 }))

/** Opens the namespace picker (keyboard shortcut, palette command). */
export function openNamespacePicker(): void {
  useNamespacePickerRequest.setState((s) => ({ seq: s.seq + 1 }))
}
