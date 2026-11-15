import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import type { Finding, SecurityResource } from '../data/types'
import { NS } from '../i18n'

/**
 * Translated texts of checks and findings. The server sends English
 * titles, messages and remediations; the locale files carry them for each
 * check id (`checks.<id>.*`), and the server's text is the fallback for a
 * check this build does not know.
 */
export function useSecurityTexts() {
  const { t } = useTranslation(NS)
  return useMemo(
    () => ({
      t,
      title: (checkId: string, fallback: string) => t(`checks.${checkId}.title`, { defaultValue: fallback }),
      remediation: (checkId: string, fallback: string) => t(`checks.${checkId}.remediation`, { defaultValue: fallback }),
      message: (f: Finding) =>
        t(`checks.${f.checkId}.message`, { defaultValue: f.message, evidence: (f.evidence ?? []).join(', ') }),
      severity: (severity: string) => t(`severity.${severity}`, { defaultValue: severity }),
      category: (category: string) => t(`category.${category}`, { defaultValue: category }),
    }),
    [t],
  )
}

/** "Deployment shop/web", "ClusterRole edit". */
export function resourceLabel(r: SecurityResource): string {
  return `${r.kind} ${r.namespace ? `${r.namespace}/` : ''}${r.name}`
}
