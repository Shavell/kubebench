import { formatDateTime } from '@/i18n/format'

/** An RFC 3339 timestamp in the UI language; a dash when there is none. */
export function DateTime({ value }: { value: string }) {
  if (!value) return <>—</>
  return <time dateTime={value}>{formatDateTime(value)}</time>
}
