/**
 * Form primitives (design §6.6). Controls are value-based (`value` +
 * `onChange(value)`) and wire themselves to a surrounding Field for label,
 * help and error.
 *
 *   <Field label={t('name')} error={nameError} required>
 *     <TextInput value={name} onChange={setName} monospace />
 *   </Field>
 */
export { Field, type FieldProps } from './Field'
export { TextInput, type TextInputProps } from './TextInput'
export { NumberInput, type NumberInputProps } from './NumberInput'
export { Select, type SelectOption, type SelectProps } from './Select'
export { Checkbox, type CheckboxProps } from './Checkbox'
export { Textarea, type TextareaProps } from './Textarea'
export { KeyValueEditor, type KeyValueEditorProps } from './KeyValueEditor'
export { keyValueErrors, pairsFromRecord, recordFromPairs, type KeyValuePair } from './keyValue'
