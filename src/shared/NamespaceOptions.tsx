import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { useNamespaceChoices } from '@/core/store/namespacePrefsStore'

/**
 * `<option>`s of a namespace `<select>` in the user's order: pinned
 * namespaces in their own group first, hidden ones left out (except the
 * `selected` one). Put the select's "all namespaces" option before it.
 */
export function NamespaceOptions({ names, selected }: { names: readonly string[]; selected?: string }) {
  const { t } = useTranslation(PLATFORM_NS)
  const { pinned, others } = useNamespaceChoices(names, selected)
  const options = (list: string[]) =>
    list.map((name) => (
      <option key={name} value={name}>
        {name}
      </option>
    ))
  if (pinned.length === 0) return <>{options(others)}</>
  return (
    <>
      <optgroup label={t('namespaces.pinnedGroup')}>{options(pinned)}</optgroup>
      {others.length > 0 && <optgroup label={t('namespaces.othersGroup')}>{options(others)}</optgroup>}
    </>
  )
}
