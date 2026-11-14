import clsx from 'clsx'
import { useEffect, useId, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { Button } from '../Button'
import { Icon } from '../Icon'
import { useField } from './fieldContext'
import { keyValueErrors, type KeyValuePair } from './keyValue'
import './form.css'

export interface KeyValueEditorProps {
  value: readonly KeyValuePair[]
  onChange: (pairs: KeyValuePair[]) => void
  /** Defaults to "Key" / "Value" (translated). */
  keyPlaceholder?: string
  valuePlaceholder?: string
  /** Label of the add button; defaults to "Add". */
  addLabel?: string
  /**
   * Extra key rule (e.g. Kubernetes label syntax): return the translated
   * problem or null. Empty keys and duplicates are always reported; use
   * keyValueErrors() with the same function to block submitting.
   */
  validateKey?: (key: string) => string | null
  disabled?: boolean
  /** Monospace keys and values (default true). */
  monospace?: boolean
}

/**
 * Editable list of key/value rows. Pair it with pairsFromRecord() and
 * recordFromPairs() for maps such as labels and annotations. Inside a Field
 * the field's label names the group.
 */
export function KeyValueEditor({
  value,
  onChange,
  keyPlaceholder,
  valuePlaceholder,
  addLabel,
  validateKey,
  disabled = false,
  monospace = true,
}: KeyValueEditorProps) {
  const { t } = useTranslation(PLATFORM_NS)
  const field = useField()
  const baseId = useId()
  const rowsRef = useRef<HTMLDivElement>(null)
  const addRef = useRef<HTMLButtonElement>(null)
  const focusNewRow = useRef(false)
  const errors = keyValueErrors(value, validateKey)

  // After "Add", put the caret into the new row's key input.
  useEffect(() => {
    if (!focusNewRow.current) return
    focusNewRow.current = false
    const keys = rowsRef.current?.querySelectorAll<HTMLInputElement>('.kb-kv__key')
    keys?.[keys.length - 1]?.focus()
  })

  const update = (index: number, patch: Partial<KeyValuePair>) =>
    onChange(value.map((pair, i) => (i === index ? { ...pair, ...patch } : pair)))

  const remove = (index: number) => {
    onChange(value.filter((_, i) => i !== index))
    // The focused remove button disappears; keep focus inside the editor.
    addRef.current?.focus()
  }

  const add = () => {
    focusNewRow.current = true
    onChange([...value, { key: '', value: '' }])
  }

  const inputClass = clsx('kb-input', monospace && 'kb-input--mono')

  return (
    <div
      role="group"
      aria-labelledby={field?.labelId}
      aria-describedby={field?.describedBy}
      className="kb-kv"
    >
      {value.length > 0 && (
        <div ref={rowsRef} className="kb-kv__rows">
          {value.map((pair, index) => {
            const error = errors[index]
            const errorId = `${baseId}error${index}`
            const row = index + 1
            return (
              // Rows have no identity of their own; inputs are controlled, so index keys are safe.
              <div key={index} className="kb-kv__row">
                <input
                  className={clsx(inputClass, 'kb-kv__key')}
                  value={pair.key}
                  placeholder={keyPlaceholder ?? t('ui.keyValue.key')}
                  aria-label={t('ui.keyValue.keyLabel', { row })}
                  aria-invalid={error ? true : undefined}
                  aria-describedby={error ? errorId : undefined}
                  spellCheck={false}
                  disabled={disabled}
                  onChange={(event) => update(index, { key: event.target.value })}
                />
                <input
                  className={inputClass}
                  value={pair.value}
                  placeholder={valuePlaceholder ?? t('ui.keyValue.value')}
                  aria-label={t('ui.keyValue.valueLabel', { row })}
                  spellCheck={false}
                  disabled={disabled}
                  onChange={(event) => update(index, { value: event.target.value })}
                />
                <button
                  type="button"
                  className="kb-kv__remove"
                  aria-label={t('ui.keyValue.remove', { row })}
                  title={t('ui.keyValue.remove', { row })}
                  disabled={disabled}
                  onClick={() => remove(index)}
                >
                  <Icon name="close" size={14} />
                </button>
                {error && (
                  <div id={errorId} className="kb-kv__error">
                    {error}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
      <Button ref={addRef} variant="secondary" icon="add" disabled={disabled} onClick={add}>
        {addLabel ?? t('ui.keyValue.add')}
      </Button>
    </div>
  )
}
