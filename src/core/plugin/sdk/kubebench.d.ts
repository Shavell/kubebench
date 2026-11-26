/**
 * KubeBench plugin SDK, API version 1 — type declarations for the
 * `window.KubeBench` global that external (runtime-loaded) UI plugins use
 * (docs/design-v0.5.md §6.8).
 *
 * Hand-written for plugin authors: copy this file into your plugin project
 * (see examples/plugins/hello-cluster) and import its types with
 * `import type`. KubeBench type-checks its own implementation
 * (src/core/plugin/globalSdk.ts) against this file, so it describes what
 * the running app provides.
 *
 * A plugin is an IIFE bundle. It must not bundle React: map `react` and
 * `react/jsx-runtime` to `KubeBench.React` and `KubeBench.jsxRuntime`, so
 * the plugin renders with the app's React instance.
 */
import type * as ReactModule from 'react'
import type * as JsxRuntimeModule from 'react/jsx-runtime'
import type { ComponentType, DependencyList, MouseEvent, ReactNode } from 'react'

// ---------------------------------------------------------------- manifest

/**
 * Display strings in a manifest (name, description, tooltips, view, command
 * and section titles) are i18n keys in the plugin's own namespace, which is
 * the plugin id; ship the bundles in `locales`. A key that is not found is
 * shown literally, so plain strings work too.
 */
export interface PluginManifest {
  /** Must equal the id in plugin.json (and the plugin's directory name). */
  id: string
  name: string
  version?: string
  description?: string
  category?: PluginCategory
  /** Backend plugin this UI needs; while it is disabled or unavailable the plugin is hidden. */
  backendPlugin?: string
  /** Translation bundles of the plugin's namespace: `{ en: {...}, tr: {...} }`. */
  locales?: Record<string, object>
  /**
   * An icon in the activity bar that opens `sidebar`. Icons are codicon
   * names, e.g. "globe". `order` sorts the entries (built-in views 0–6,
   * Extensions 90); without it the entry goes after the built-in views.
   */
  activityBar?: { icon: string; tooltip: string; order?: number }
  sidebar?: { id: string; title: string; component: ComponentType }
  /** Components for editor tabs; open them with `layout.openTab({ kind, … })`. */
  tabRenderers?: { kind: string; component: ComponentType<{ tab: TabDescriptor }> }[]
  bottomPanelTabs?: { id: string; title: string; component: ComponentType; order?: number }[]
  statusBarItems?: { id: string; alignment: 'left' | 'right'; priority?: number; component: ComponentType }[]
  /** Command palette entries (F1). */
  commands?: { id: string; title: string; icon?: string; run: () => void }[]
  /** Entries in the context menus and detail toolbars of resources of these kinds. */
  resourceActions?: ResourceAction[]
  /** Sections on the Settings page. */
  settingsSections?: { id: string; title: string; component: ComponentType }[]
}

export type PluginCategory =
  | 'core'
  | 'workloads'
  | 'gitops'
  | 'storage'
  | 'security'
  | 'observability'
  | 'tools'

export interface ResourceRef {
  kind: string
  apiVersion?: string
  namespace?: string
  name: string
}

export interface ResourceAction {
  id: string
  title: string
  icon?: string
  /** e.g. ['Deployment', 'Service'], or ['*'] for every kind. */
  kinds: string[]
  /** Needed on the active cluster (default 'read'); the action is disabled without it. */
  permission?: 'read' | 'write' | 'exec'
  run: (ref: ResourceRef) => void
}

/** An editor tab. Tabs are plain data; `kind` selects the renderer. */
export interface TabDescriptor {
  /** Unique; opening a tab whose id is already open focuses it. */
  id: string
  title: string
  /**
   * i18n key (in namespace `titleNs`) the title is shown from, so it follows
   * language changes; `titleValues` fills its interpolation variables.
   */
  titleKey?: string
  titleNs?: string
  titleValues?: Record<string, string | number>
  icon?: string
  kind: string
  props?: Record<string, unknown>
}

// ---------------------------------------------------------------- data

/** Effective permissions of the signed-in user on the active cluster. */
export interface SessionPermissions {
  role: 'admin' | 'operator' | 'viewer'
  canRead: boolean
  canWrite: boolean
  canExec: boolean
  isAdmin: boolean
  /** The active cluster is read-only: writes and exec are refused for everyone. */
  readOnly: boolean
}

export interface QueryState<T> {
  data: T | null
  loading: boolean
  /** Message of the last failure; null after a success. */
  error: string | null
  /** Machine code of the last failure (ApiError.code), e.g. "forbidden". */
  errorCode: string | null
  /** Reason code of the last failure, e.g. "kubernetes.forbidden_namespace" (design §3.3). */
  errorReason: string | null
}

/** What a query reads, so that live updates refresh it when that changes. */
export interface QueryScope {
  /** Resources as watch topics: `pods`, `deployments`, `applications.argoproj.io`, … */
  topics?: readonly string[]
  /** The one namespace read; absent = all namespaces (or cluster-scoped). */
  namespace?: string
}

export type QueryValue = string | number | boolean | null | undefined
/** Query parameters; every value is URL-encoded, undefined/null/'' are left out. */
export type QueryParams = Record<string, QueryValue>

export interface ApiRequestInit extends Omit<RequestInit, 'body'> {
  /** Sent as JSON unless it is a string, FormData, Blob or URLSearchParams. */
  body?: unknown
  params?: QueryParams
}

/**
 * What every API helper rejects with: `status` is the HTTP status (0 when
 * the server is unreachable), `code` the machine code from the server's
 * `{"error","code"}` body. `reason` and `params` are the stable reason
 * code of an error KubeBench explains itself (e.g. "cluster.read_only"),
 * which `notify.error` shows in the user's language; `message` stays the
 * server's English text.
 */
export interface ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly reason?: string
  readonly params?: Record<string, unknown>
}

/** Client for a backend plugin's routes under `/api/plugins/<id>`. */
export interface PluginApi {
  get<T>(path: string, params?: QueryParams): Promise<T>
  post<T>(path: string, body?: unknown, params?: QueryParams): Promise<T>
  put<T>(path: string, body?: unknown, params?: QueryParams): Promise<T>
  patch<T>(path: string, body?: unknown, params?: QueryParams): Promise<T>
  del<T>(path: string, params?: QueryParams): Promise<T>
}

// ---------------------------------------------------------------- ui

export interface TableColumn<T> {
  key: string
  header: string
  width?: string
  render: (row: T) => ReactNode
  /** Makes the column sortable by clicking its header. */
  sortValue?: (row: T) => string | number
}

export interface TableProps<T> {
  columns: TableColumn<T>[]
  rows: T[]
  rowKey: (row: T) => string
  onRowClick?: (row: T) => void
  onRowContextMenu?: (row: T, event: MouseEvent) => void
}

export type BadgeTone = 'success' | 'warning' | 'error' | 'info' | 'neutral'

export interface BadgeProps {
  children: string
  /** Derived from common Kubernetes statuses ("Running", "Failed", …) when omitted. */
  tone?: BadgeTone
}

export interface EmptyStateProps {
  icon: string
  title: string
  hint?: string
}

/** Header strip of a page: title on the left, actions on the right. */
export interface ToolbarProps {
  title: string
  children?: ReactNode
}

export interface DialogProps {
  open: boolean
  title: ReactNode
  /** Esc, the close button and (when enabled) backdrop clicks call this; not while busy. */
  onClose: () => void
  children?: ReactNode
  description?: ReactNode
  /** Buttons, right-aligned; the primary action last. */
  footer?: ReactNode
  icon?: string
  size?: 'small' | 'medium' | 'large'
  /** Makes the dialog a form: Enter or a type="submit" button calls it. */
  onSubmit?: () => void
  busy?: boolean
  closeOnBackdropClick?: boolean
  role?: 'dialog' | 'alertdialog'
}

export interface ConfirmDialogProps {
  open: boolean
  title: ReactNode
  message: ReactNode
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  /** Typed by the user before confirming on production clusters (always with `requirePhrase`). */
  confirmPhrase?: string
  requirePhrase?: boolean
  /** A returned promise keeps the dialog busy; a rejection is shown and the dialog stays open. */
  onConfirm: () => void | Promise<unknown>
  onClose: () => void
  children?: ReactNode
}

export interface CodeEditorProps {
  value: string
  onChange?: (value: string) => void
  language?: 'yaml' | 'json' | 'plaintext' | 'shell' | 'markdown'
  readOnly?: boolean
  ariaLabel?: string
  minimap?: boolean
  lineNumbers?: boolean
  wordWrap?: boolean
  placeholder?: string
  autoFocus?: boolean
  /** CSS height (default 100%). */
  height?: number | string
  className?: string
}

export interface ToastOptions {
  detail?: string
  /** Milliseconds; 0 keeps the toast until it is dismissed. */
  timeout?: number
  action?: { label: string; run: () => void }
}

export interface Notify {
  success(message: string, options?: ToastOptions): number
  info(message: string, options?: ToastOptions): number
  /** A message, or a thrown value (an ApiError gets a friendly translated headline). */
  error(messageOrError: unknown, options?: ToastOptions): number
  dismiss(id: number): void
}

// ---------------------------------------------------------------- the SDK

export type TranslateFunction = (key: string, options?: Record<string, unknown>) => string

export interface KubeBenchSdk {
  readonly apiVersion: 1
  /** Version of the running KubeBench. */
  readonly version: string
  readonly React: typeof ReactModule
  readonly jsxRuntime: typeof JsxRuntimeModule
  /**
   * Registers the plugin. Call it once, synchronously, when the bundle
   * runs; the manifest id must be the id of the installed plugin.
   */
  registerPlugin(manifest: PluginManifest): void
  readonly i18n: {
    /** Translates in the current language; pass `{ ns: '<plugin id>' }` for the plugin's keys. */
    t: TranslateFunction
    /** Adds bundles to a namespace (manifest `locales` does this for the plugin's own). */
    registerTranslations(ns: string, bundles: Record<string, object>): void
    /** Hook: `t` bound to a namespace; the component re-renders when the language changes. */
    useTranslation(ns: string): TranslateFunction
  }
  readonly api: {
    /** JSON request to `/api<path>`; rejects with ApiError. */
    apiFetch<T>(path: string, init?: ApiRequestInit): Promise<T>
    pluginApi(pluginId: string): PluginApi
    /** Read-only Kubernetes API, e.g. `k8sGet('/api/v1/namespaces')`. Secrets are not readable this way. */
    k8sGet<T = unknown>(path: string, params?: QueryParams): Promise<T>
  }
  readonly data: {
    /**
     * Runs `fn` again when `deps` change or after `refreshData()`; with
     * `scope.topics`, also when live updates report a change of them.
     */
    usePluginQuery<T>(fn: () => Promise<T>, deps: DependencyList, scope?: QueryScope): QueryState<T>
    /** Reloads every query, e.g. after a change. */
    refreshData(): void
    /** Reloads the queries of changed topics (namespace '' = all of the topic's queries). */
    refreshTopics(changes: readonly { topic: string; namespace: string }[]): void
  }
  readonly layout: {
    openTab(tab: TabDescriptor): void
  }
  readonly permissions: {
    usePermissions(): SessionPermissions
  }
  readonly ui: {
    Table: <T>(props: TableProps<T>) => ReactNode
    Badge: ComponentType<BadgeProps>
    EmptyState: ComponentType<EmptyStateProps>
    Toolbar: ComponentType<ToolbarProps>
    Dialog: ComponentType<DialogProps>
    ConfirmDialog: ComponentType<ConfirmDialogProps>
    CodeEditor: ComponentType<CodeEditorProps>
    notify: Notify
  }
}

declare global {
  /** Installed by KubeBench before any external plugin script runs. */
  var KubeBench: KubeBenchSdk
}
