import * as React from 'react'
import * as jsxRuntime from 'react/jsx-runtime'
import { useTranslation } from 'react-i18next'
import { usePermissions } from '@/core/permissions'
import { useLayoutStore } from '@/core/store/layoutStore'
import { apiFetch, type QueryParams } from '@/data/apiClient'
import { refreshData } from '@/data/DataContext'
import { appVersion } from '@/data/mode'
import { pluginApi, usePluginQuery } from '@/data/pluginData'
import type { DataProviders } from '@/data/providers'
import { i18n, registerTranslations } from '@/i18n'
import { Badge } from '@/shared/Badge'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { Dialog } from '@/shared/Dialog'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { EmptyState } from '@/shared/EmptyState'
import { notify } from '@/shared/notify'
import { Table } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { registerExternalPlugin } from './externalPlugins'
import type { KubeBenchSdk, TranslateFunction } from './sdk/kubebench'

const translate: TranslateFunction = (key, options) => i18n.t(key, options ?? {})

/** `t` of a namespace, re-rendering the component on language changes. */
function useNamespaceTranslation(ns: string): TranslateFunction {
  const { t } = useTranslation(ns)
  return (key, options) => t(key, options ?? {})
}

/**
 * Installs `window.KubeBench`, the SDK of external plugins (design §6.8;
 * types in ./sdk/kubebench.d.ts). Called at boot, after the session is
 * known and before any plugin script is injected.
 */
export function installGlobalSdk(providers: DataProviders): void {
  const sdk: KubeBenchSdk = {
    apiVersion: 1,
    version: appVersion(),
    React,
    jsxRuntime,
    registerPlugin: registerExternalPlugin,
    i18n: Object.freeze({
      t: translate,
      registerTranslations,
      useTranslation: useNamespaceTranslation,
    }),
    api: Object.freeze({
      apiFetch,
      pluginApi,
      k8sGet: <T>(path: string, params?: QueryParams) => providers.cluster.k8sGet<T>(path, params),
    }),
    data: Object.freeze({ usePluginQuery, refreshData }),
    layout: Object.freeze({ openTab: (tab) => useLayoutStore.getState().openTab(tab) }),
    permissions: Object.freeze({ usePermissions }),
    ui: Object.freeze({ Table, Badge, EmptyState, Toolbar, Dialog, ConfirmDialog, CodeEditor, notify }),
  }
  window.KubeBench = Object.freeze(sdk)
}
