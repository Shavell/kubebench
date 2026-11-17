import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Select } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import type { BrowserPort, BrowserTarget, Scheme } from '../data/types'
import { NS } from '../i18n'
import { normalizePath, portLabel } from '../model/ports'

interface AddressBarProps {
  /** The target's in-cluster address, e.g. http://web.shop:80. */
  address: string
  /** The path the frame shows. */
  path: string
  ports: BrowserPort[]
  target: BrowserTarget
  canBack: boolean
  canForward: boolean
  /** The page's URL for a browser tab of its own; null in demo mode. */
  externalUrl: string | null
  onBack: () => void
  onForward: () => void
  onReload: () => void
  onNavigate: (path: string) => void
  onTargetChange: (port: number, scheme: Scheme) => void
}

const SCHEMES: Scheme[] = ['http', 'https']

function IconButton({ icon, label, disabled, onClick }: { icon: string; label: string; disabled?: boolean; onClick: () => void }) {
  return (
    <button type="button" className="sb-icon" title={label} aria-label={label} disabled={disabled} onClick={onClick}>
      <Icon name={icon} size={16} />
    </button>
  )
}

/** Back, forward, reload, the in-cluster address with an editable path, the port and "open in new tab". */
export function AddressBar(props: AddressBarProps) {
  const { t } = useTranslation(NS)
  // The path being typed; null while the bar follows the frame.
  const [draft, setDraft] = useState<string | null>(null)
  const { target } = props

  return (
    <div className="sb-bar" role="toolbar" aria-label={t('toolbar.label')}>
      <IconButton icon="arrow-left" label={t('toolbar.back')} disabled={!props.canBack} onClick={props.onBack} />
      <IconButton icon="arrow-right" label={t('toolbar.forward')} disabled={!props.canForward} onClick={props.onForward} />
      <IconButton icon="refresh" label={t('toolbar.reload')} onClick={props.onReload} />
      <form
        className="sb-address"
        onSubmit={(e) => {
          e.preventDefault()
          if (draft !== null) props.onNavigate(normalizePath(draft))
          setDraft(null)
        }}
      >
        <span className="sb-address__origin" title={props.address}>
          {props.address}
        </span>
        <input
          className="sb-address__path"
          aria-label={t('toolbar.path')}
          spellCheck={false}
          value={draft ?? props.path}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={() => setDraft(null)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setDraft(null)
          }}
        />
      </form>
      <Select
        className="sb-select"
        aria-label={t('toolbar.port')}
        value={Number(target.port)}
        options={props.ports.map((p) => ({ value: p.port, label: portLabel(p) }))}
        onChange={(port) => props.onTargetChange(port, props.ports.find((p) => p.port === port)?.scheme ?? 'http')}
      />
      <Select
        className="sb-select"
        aria-label={t('toolbar.scheme')}
        value={target.scheme}
        options={SCHEMES.map((s) => ({ value: s, label: s }))}
        onChange={(scheme) => props.onTargetChange(Number(target.port), scheme)}
      />
      <IconButton
        icon="link-external"
        label={props.externalUrl ? t('toolbar.newTab') : t('toolbar.newTabDemo')}
        disabled={!props.externalUrl}
        onClick={() => {
          if (props.externalUrl) window.open(props.externalUrl, '_blank', 'noopener,noreferrer')
        }}
      />
    </div>
  )
}
