import { Fragment, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useManifestText } from '@/i18n'
import type { Extension } from './catalog'
import { EXTENSIONS_NS } from './namespace'

/** What a plugin adds to the workbench, from its manifest. */
export function ExtensionContributions({ extension }: { extension: Extension }) {
  const { t } = useTranslation(EXTENSIONS_NS)
  const text = useManifestText()
  const { manifest, id } = extension

  if (!manifest) {
    const key = extension.source === 'external' ? 'contributes.notLoaded' : 'contributes.serverOnly'
    return <p className="ext-detail__description">{t(key)}</p>
  }

  const rows: { label: string; value: ReactNode }[] = []
  const list = (items: string[]) => (
    <ul className="ext-detail__list">
      {items.map((item) => (
        <li key={item}>{item}</li>
      ))}
    </ul>
  )
  if (manifest.activityBar) {
    rows.push({ label: t('contributes.activityBar'), value: text(id, manifest.activityBar.tooltip) })
  }
  if (manifest.sidebar) rows.push({ label: t('contributes.sidebar'), value: text(id, manifest.sidebar.title) })
  if (manifest.tabRenderers?.length) {
    rows.push({
      label: t('contributes.tabs'),
      value: t('contributes.tabKinds', { count: manifest.tabRenderers.length }),
    })
  }
  if (manifest.commands?.length) {
    rows.push({ label: t('contributes.commands'), value: list(manifest.commands.map((c) => text(id, c.title))) })
  }
  if (manifest.bottomPanelTabs?.length) {
    rows.push({ label: t('contributes.panels'), value: list(manifest.bottomPanelTabs.map((p) => text(id, p.title))) })
  }
  if (manifest.statusBarItems?.length) {
    rows.push({
      label: t('contributes.statusBar'),
      value: t('contributes.statusBarItems', { count: manifest.statusBarItems.length }),
    })
  }
  if (manifest.resourceActions?.length) {
    rows.push({
      label: t('contributes.resourceActions'),
      value: list(
        manifest.resourceActions.map((a) =>
          t('contributes.resourceAction', { title: text(id, a.title), kinds: a.kinds.join(', ') }),
        ),
      ),
    })
  }
  if (manifest.settingsSections?.length) {
    rows.push({ label: t('contributes.settings'), value: list(manifest.settingsSections.map((s) => text(id, s.title))) })
  }

  if (rows.length === 0) return <p className="ext-detail__description">{t('contributes.nothing')}</p>
  return (
    <dl className="ext-detail__facts">
      {rows.map((row) => (
        <Fragment key={row.label}>
          <dt>{row.label}</dt>
          <dd>{row.value}</dd>
        </Fragment>
      ))}
    </dl>
  )
}
