import { createContext, useContext } from 'react'

/** What a Field hands to the control rendered inside it. */
export interface FieldContextValue {
  /** id of the control: the target of the field's <label htmlFor>. */
  controlId: string
  /** id of the label, for group controls that need aria-labelledby. */
  labelId: string
  /** ids of the error and help texts, for aria-describedby. */
  describedBy: string | undefined
  invalid: boolean
  required: boolean
}

export const FieldContext = createContext<FieldContextValue | null>(null)

export function useField(): FieldContextValue | null {
  return useContext(FieldContext)
}

/**
 * id and ARIA props for a single control. Inside a Field they come from the
 * field (its label, help and error); outside one only `ownId` is used.
 */
export function useFieldControlProps(ownId?: string): {
  id?: string
  'aria-describedby'?: string
  'aria-invalid'?: true
  'aria-required'?: true
} {
  const field = useField()
  if (!field) return ownId ? { id: ownId } : {}
  return {
    id: field.controlId,
    'aria-describedby': field.describedBy,
    'aria-invalid': field.invalid || undefined,
    'aria-required': field.required || undefined,
  }
}
