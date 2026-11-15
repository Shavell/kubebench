import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { CORE_NS } from '@/i18n'
import { formatNumber } from '@/i18n/format'
import { describeError } from '@/shared/apiErrorText'
import { Select } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { NS } from '../i18n'
import { ToolButton } from '../ToolButton'
import { TAIL_SIZES, useLogPreferences } from './logPreferences'
import type { LogStreamStatus } from './useLogStream'

export interface LogsToolbarProps {
  /** What the view follows, e.g. "shop/web-1" or "Deployment web". */
  label: string
  /** Shown on hover over the label, e.g. the label selector. */
  labelTitle?: string
  /** The followed containers and matching pods, for streams of several pods. */
  pods?: { containers: string[]; matched: number }
  status: LogStreamStatus
  paused: boolean
  /** Lines that arrived while paused. */
  newer: number
  shown: number
  total: number
  filter: string
  onFilter: (filter: string) => void
  /** Container picker, for one pod. */
  container?: { value: string; options: string[]; onChange: (container: string) => void }
  previous: boolean
  onPrevious: (previous: boolean) => void
  onTail: (tail: number) => void
  onPause: () => void
  onDownload: () => void
  onClear: () => void
  onReconnect: () => void
}

/** The log view's toolbar: what is followed and how it goes, then the view options. */
export function LogsToolbar(props: LogsToolbarProps) {
  const { t } = useTranslation(NS)
  const prefs = useLogPreferences()
  const { pods } = props

  return (
    <div className="kb-logs__toolbar">
      <div className="kb-logs__summary">
        <Icon name="output" size={14} />
        <span className="kb-logs__label" title={props.labelTitle}>
          {props.label}
        </span>
        {pods && pods.containers.length > 0 && (
          <span className="kb-logs__pods" title={pods.containers.join('\n')}>
            {pods.matched > pods.containers.length
              ? t('logs.podsOf', { count: pods.matched, shown: formatNumber(pods.containers.length), total: formatNumber(pods.matched) })
              : t('logs.pods', { count: pods.containers.length, pods: formatNumber(pods.containers.length) })}
          </span>
        )}
        <StreamStatus status={props.status} paused={props.paused} newer={props.newer} />
        <span className="kb-logs__count">
          {props.shown === props.total
            ? t('logs.lines', { count: props.total, lines: formatNumber(props.total) })
            : t('logs.linesFiltered', { count: props.total, shown: formatNumber(props.shown), total: formatNumber(props.total) })}
        </span>
      </div>
      <div className="kb-logs__controls">
        <input
          className="kb-logs__filter"
          type="search"
          placeholder={t('common.filter', { ns: CORE_NS })}
          aria-label={t('logs.filterLabel')}
          value={props.filter}
          onChange={(event) => props.onFilter(event.target.value)}
          spellCheck={false}
        />
        {props.container && (
          <Select
            className="kb-logs__select kb-logs__select--container"
            aria-label={t('logs.container')}
            title={t('logs.container')}
            value={props.container.value}
            options={props.container.options.map((name) => ({ value: name, label: name }))}
            onChange={props.container.onChange}
          />
        )}
        <Select
          className="kb-logs__select"
          aria-label={t('logs.tail')}
          title={t('logs.tail')}
          value={prefs.tail}
          options={TAIL_SIZES.map((size) => ({
            value: size,
            label: size ? t('logs.tailLines', { count: size, lines: formatNumber(size) }) : t('logs.tailAll'),
          }))}
          onChange={props.onTail}
        />
        <ToolButton
          icon="history"
          title={t('logs.previous')}
          pressed={props.previous}
          onClick={() => props.onPrevious(!props.previous)}
        />
        <ToolButton
          icon="watch"
          title={t('logs.timestamps')}
          pressed={prefs.timestamps}
          onClick={() => prefs.set('timestamps', !prefs.timestamps)}
        />
        <ToolButton icon="word-wrap" title={t('logs.wrap')} pressed={prefs.wrap} onClick={() => prefs.set('wrap', !prefs.wrap)} />
        <ToolButton
          icon="symbol-color"
          title={t('logs.colorize')}
          pressed={prefs.colorize}
          onClick={() => prefs.set('colorize', !prefs.colorize)}
        />
        <span className="kb-logs__separator" />
        <ToolButton
          icon={props.paused ? 'play' : 'debug-pause'}
          title={props.paused ? t('logs.follow') : t('logs.pause')}
          onClick={props.onPause}
        />
        <ToolButton icon="desktop-download" title={t('logs.download')} onClick={props.onDownload} />
        <ToolButton icon="clear-all" title={t('logs.clear')} onClick={props.onClear} />
        <ToolButton icon="debug-restart" title={t('logs.reconnect')} onClick={props.onReconnect} />
      </div>
    </div>
  )
}

function StreamStatus({ status, paused, newer }: { status: LogStreamStatus; paused: boolean; newer: number }) {
  const { t } = useTranslation(NS)
  const view = ((): { icon: string; tone: 'ok' | 'busy' | 'muted' | 'bad'; text: string; title?: string } => {
    switch (status.state) {
      case 'connecting':
        return { icon: 'loading', tone: 'busy', text: t('logs.status.connecting') }
      case 'live':
        return paused
          ? { icon: 'debug-pause', tone: 'muted', text: t('logs.status.paused', { count: newer, lines: formatNumber(newer) }) }
          : { icon: 'circle-filled', tone: 'ok', text: t('logs.status.live') }
      case 'ended':
        return { icon: 'pass', tone: 'muted', text: t('logs.status.ended') }
      case 'retrying': {
        const reason = describeError(status.error)
        return {
          icon: 'debug-disconnect',
          tone: 'bad',
          text: t('logs.status.retrying', { seconds: formatNumber(Math.round(status.delayMs / 1000)) }),
          title: [reason.message, reason.detail].filter(Boolean).join(': '),
        }
      }
      case 'failed': {
        // The headline; the server's reason is the tooltip (and the body, while there are no lines).
        const reason = describeError(status.error)
        return { icon: 'error', tone: 'bad', text: reason.message, title: reason.detail }
      }
    }
  })()
  return (
    <span className={clsx('kb-logs__status', `kb-logs__status--${view.tone}`)} role="status" title={view.title}>
      <Icon name={view.icon} size={14} className={status.state === 'connecting' ? 'codicon-modifier-spin' : undefined} />
      {view.text}
    </span>
  )
}
