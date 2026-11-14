import { registry } from '@/core/plugin/registry'
import { useRegistryRevision } from '@/core/plugin/useRegistry'
import { useManifestText } from '@/i18n'
import { SettingsSection } from '../SettingsSection'

/**
 * Sections contributed by active plugins (`PluginManifest.settingsSections`).
 * Titles are i18n keys in the contributing plugin's namespace; the list
 * follows plugins being enabled or disabled.
 */
export function PluginSettingsSections() {
  useRegistryRevision()
  const manifestText = useManifestText()

  return registry.settingsSections().map((section) => {
    const Body = section.component
    return (
      <SettingsSection key={`${section.pluginId}/${section.id}`} title={manifestText(section.pluginId, section.title)}>
        <Body />
      </SettingsSection>
    )
  })
}
