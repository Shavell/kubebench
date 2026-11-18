import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Icon } from '@/shared/Icon'
import { NS } from '../i18n'
import { normalizePath } from '../model/ports'

interface AddressBarProps {
  /** The target's address, e.g. http://web.shop:80. */
  address: string
  /** Shown on the address, e.g. the Service an Ingress route leads to. */
  addressTitle?: string
  /** The path the frame shows. */
  path: string
  /** The pickers of the target (port or Ingress route, and scheme). */
  pickers: ReactNode
  canBack: boolean
  canForward: boolean
  /** The page's URL for a browser tab of its own; null in demo mode. */
  externalUrl: string | null
  /** An OpenShift Route's public URL, served by its router (design §7.15); null otherwise. */
  publicUrl?: string | null
  onBack: () => void
  onForward: () => void
  onReload: () => void
  onNavigate: (path: string) => void
}

function IconButton({ icon, label, disabled, onClick }: { icon: string; label: string; disabled?: boolean; onClick: () => void }) {
  return (
    <button type="button" className="sb-icon" title={label} aria-label={label} disabled={disabled} onClick={onClick}>
      <Icon name={icon} size={16} />
    </button>
  )
}

/** Back, forward, reload, the address with an editable path, the target's pickers and "open in new tab". */
export function AddressBar(props: AddressBarProps) {
  const { t } = useTranslation(NS)
  // The path being typed; null while the bar follows the frame.
  const [draft, setDraft] = useState<string | null>(null)

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
        <span className="sb-address__origin" title={props.addressTitle ?? props.address}>
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
      {props.pickers}
      {props.publicUrl && (
        <IconButton
          icon="globe"
          label={t('route.openPublic', { url: props.publicUrl })}
          onClick={() => window.open(props.publicUrl!, '_blank', 'noopener,noreferrer')}
        />
      )}
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
