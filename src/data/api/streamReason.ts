import { isReasonMessage, reasonMessageText } from '@/core/reasons'

/**
 * The text of why the server ended a stream or reported a failing log, in
 * the UI language: the data of a log stream's `event: error` and a watch
 * stream's `event: end` is a reason message (design §3.3,
 * `{"error", "reason"?, "params"?}`). Anything else is shown as it is.
 */
export function streamReasonText(data: string): string {
  let parsed: unknown
  try {
    parsed = JSON.parse(data)
  } catch {
    return data
  }
  return isReasonMessage(parsed) ? reasonMessageText(parsed) : data
}
