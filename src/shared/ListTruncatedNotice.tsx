import { useTranslation } from 'react-i18next'
import type { ListTruncation } from '@/data/types'
import { formatNumber } from '@/i18n/format'
import { Button } from './Button'
import { Icon } from './Icon'
import { LIST_PAGE } from './listLimit'
import './ListTruncatedNotice.css'

interface ListTruncatedNoticeProps {
  /** The list's `truncated`; nothing is shown while it is absent. */
  truncated: ListTruncation | undefined
  /** How many objects are loaded. */
  loaded: number
  /** Raises the list's limit (useListLimit). */
  onLoadMore: () => void
}

/**
 * Above a list cut at its limit (design §4.4, "Lists"): how many objects
 * are loaded and left out, that search, sort and filters cover the loaded
 * ones only, and a button that loads LIST_PAGE more.
 */
export function ListTruncatedNotice({ truncated, loaded, onLoadMore }: ListTruncatedNoticeProps) {
  const { t } = useTranslation()
  if (!truncated) return null
  const text =
    truncated.remaining === null
      ? t('lists.truncatedUnknown', { loaded: formatNumber(loaded) })
      : t('lists.truncated', { loaded: formatNumber(loaded), remaining: formatNumber(truncated.remaining) })
  return (
    <div className="kb-list-truncated" role="status">
      <Icon name="info" size={14} />
      <span className="kb-list-truncated__text">
        {text} {t('lists.loadedOnly')}
      </span>
      <Button variant="secondary" icon="fold-down" onClick={onLoadMore}>
        {t('lists.loadMore', { page: formatNumber(LIST_PAGE) })}
      </Button>
    </div>
  )
}
