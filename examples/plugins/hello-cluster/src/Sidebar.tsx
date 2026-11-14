import { openNamespacesTab, PLUGIN_ID } from './plugin'

/** The plugin's side bar view: what it does, and a button to open its tab. */
export function Sidebar() {
  const t = window.KubeBench.i18n.useTranslation(PLUGIN_ID)
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: '8px 12px' }}>
      <p style={{ margin: 0, fontSize: 12, lineHeight: 1.5, color: 'var(--vscode-descriptionForeground)' }}>
        {t('sidebar.intro')}
      </p>
      <button type="button" className="kb-button" style={{ alignSelf: 'flex-start' }} onClick={openNamespacesTab}>
        {t('sidebar.open')}
      </button>
    </div>
  )
}
