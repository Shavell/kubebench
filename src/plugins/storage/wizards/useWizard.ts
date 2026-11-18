import { useEffect, useMemo, useState } from 'react'
import { warningTexts } from '@/core/reasons'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData } from '@/data/DataContext'
import { notify } from '@/shared/notify'
import type { CreateResult } from '../data/types'
import { translate } from '../i18n'
import { toYaml, type Manifest } from '../model/build'
import type { Problem, Problems } from '../model/rules'
import { openResource } from '../tabs'

export type WizardStatus =
  | { state: 'idle' }
  | { state: 'busy'; dryRun: boolean }
  | { state: 'valid'; yaml: string; warnings: string[] }
  | { state: 'failed'; yaml: string; error: unknown }

/**
 * The Validate → Create flow shared by the wizards. `object` is what the
 * form describes (the YAML preview); `submit` sends the request, as a
 * server-side dry run for Validate. Field problems are shown once the user
 * tried to submit; a successful Create closes the wizard and opens the new
 * object. The tab is marked dirty while the form differs from its start.
 */
export function useWizard({
  tabId,
  object,
  problems,
  dirty,
  submit,
}: {
  tabId: string
  object: Manifest
  problems: Problems
  dirty: boolean
  submit: (dryRun: boolean) => Promise<CreateResult>
}) {
  const [attempted, setAttempted] = useState(false)
  const [status, setStatus] = useState<WizardStatus>({ state: 'idle' })
  const yaml = useMemo(() => toYaml(object), [object])
  const invalid = Object.keys(problems).length > 0

  useEffect(() => {
    useLayoutStore.getState().setTabDirty(tabId, dirty)
  }, [tabId, dirty])

  const run = async (dryRun: boolean) => {
    setAttempted(true)
    if (invalid) return
    setStatus({ state: 'busy', dryRun })
    let result: CreateResult
    try {
      result = await submit(dryRun)
    } catch (error) {
      setStatus({ state: 'failed', yaml, error })
      return
    }
    if (dryRun) {
      setStatus({ state: 'valid', yaml, warnings: warningTexts(result.warnings, result.warningReasons) })
      return
    }
    const open = () => openResource(result.kind, result.namespace, result.name)
    notify.success(translate('wizard.created', { kind: result.kind, name: result.name }), {
      detail: warningTexts(result.warnings, result.warningReasons).join('\n') || undefined,
      action: { label: translate('wizard.open'), run: open },
    })
    refreshData()
    const layout = useLayoutStore.getState()
    layout.setTabDirty(tabId, false)
    layout.closeTab(tabId)
    open()
  }

  // A result only holds for the YAML it was given for.
  const stale = (status.state === 'valid' || status.state === 'failed') && status.yaml !== yaml
  const shownStatus: WizardStatus = stale ? { state: 'idle' } : status

  return {
    yaml,
    status: shownStatus,
    busy: status.state === 'busy',
    /** The translated problem of a field, once the user tried to submit. */
    problemOf: (field: string): string | null => (attempted ? problemText(problems[field]) : null),
    validate: () => void run(true),
    create: () => void run(false),
  }
}

export function problemText(problem: Problem | undefined): string | null {
  return problem ? translate(`validation.${problem.key}`, problem.values ?? {}) : null
}
