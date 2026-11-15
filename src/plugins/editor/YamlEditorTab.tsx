import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import type { TabDescriptor } from '@/core/plugin/types'
import type { ObjectRef } from '@/data/types'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { ResourceYamlEditor } from './ResourceYamlEditor'

/**
 * Renderer of the "yaml" tab kind (design §7.2; core/plugin/editorTabs
 * opens it). With `props.ref` ({apiVersion, kind, namespace, name}) the
 * tab views, edits and applies that cluster object. Without one it shows
 * `props.text`, as in v0.4: `readOnly` defaults to true, and
 * `readOnly: false` allows local edits that are never applied.
 */
export function YamlEditorTab({ tab }: { tab: TabDescriptor }) {
  const target = objectRefOf(tab.props?.ref)
  // Keyed by tab: two YAML tabs must not share the editor's local state.
  if (target) return <ResourceYamlEditor key={tab.id} tabId={tab.id} target={target} />
  return <TextTab tab={tab} />
}

function objectRefOf(value: unknown): ObjectRef | null {
  if (typeof value !== 'object' || value === null) return null
  const { apiVersion, kind, namespace, name } = value as Record<string, unknown>
  if (typeof apiVersion !== 'string' || typeof kind !== 'string' || typeof name !== 'string') return null
  if (!apiVersion || !kind || !name) return null
  return { apiVersion, kind, name, namespace: typeof namespace === 'string' && namespace ? namespace : null }
}

function TextTab({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(PLATFORM_NS)
  const text = typeof tab.props?.text === 'string' ? tab.props.text : ''
  const readOnly = tab.props?.readOnly !== false

  return (
    <CodeEditor
      value={text}
      language="yaml"
      readOnly={readOnly}
      minimap
      ariaLabel={t('editor.yamlTab.ariaLabel', { title: tab.title })}
    />
  )
}
