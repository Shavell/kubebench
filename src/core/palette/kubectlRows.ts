import { splitKindName, VERBS, type Alias, type KubectlCommand, type Verb } from './kubectl.ts'

/**
 * Palette rows for a parsed kubectl-style command (pure: data, permissions,
 * translations and actions come in through `KubectlEnv`, so it is unit
 * tested with fakes in kubectlRows.test.ts). Enter runs a row, Tab puts its
 * `complete` text into the input.
 */
export interface PaletteRow {
  id: string
  label: string
  detail?: string
  icon: string
  /** Runs the row (Enter, click); rows without it only complete. */
  run?: () => void
  /** Replaces the input on Tab (and on Enter when there is nothing to run). */
  complete?: string
  /** Shown instead of running: why the row cannot be used. */
  disabledReason?: string
  /** Shortcut caps shown at the end of the row, e.g. ['Ctrl', 'B']. */
  keys?: string[]
}

export interface KindInfo {
  kind: string
  names: string[]
  namespaced: boolean
  icon?: string
  openList: (namespace?: string) => void
  openDetail: (object: { namespace?: string; name: string }) => void
}

export interface PodInfo {
  namespace: string
  name: string
  containers: string[]
}

export interface ClusterInfo {
  name: string
  active: boolean
  /** Bookmarks and broken entries cannot be switched to. */
  disabledReason?: string
}

export interface PodTarget {
  namespace: string
  pod: string
  container: string
}

export interface KubectlEnv {
  kinds: readonly KindInfo[]
  /** null while loading (or not needed). */
  namespaces: readonly string[] | null
  pods: readonly PodInfo[] | null
  objects: readonly { namespace?: string; name: string }[] | null
  clusters: readonly ClusterInfo[] | null
  /** The global namespace filter ('' = all namespaces). */
  currentNamespace: string
  /** Why the data the command needs could not be loaded. */
  loadError: string | null
  /** Why a permission is not held, or null when it is. */
  denied: (permission: 'read' | 'exec') => string | null
  t: (key: string, values?: Record<string, unknown>) => string
  actions: {
    setNamespace: (namespace: string) => void
    showLogs: (target: PodTarget) => void
    openShell: (target: PodTarget) => void
    switchCluster: (name: string) => void
  }
}

/** Data a command needs before its rows can be built. */
export interface KubectlNeeds<K extends KindInfo = KindInfo> {
  namespaces: boolean
  clusters: boolean
  /** Pods in this namespace ('' = all). */
  pods?: { namespace: string }
  /** Objects of this kind ('' = all namespaces). */
  objects?: { kind: K; namespace: string }
}

/** Most rows a command lists; type more to narrow them down. */
export const MAX_ROWS = 50

export function findKind<K extends KindInfo>(kinds: readonly K[], name: string): K | undefined {
  const lower = name.toLowerCase()
  return kinds.find((k) => k.names.includes(lower) || k.kind.toLowerCase() === lower)
}

/** The namespace a command acts in: `-n`, else the global filter ('' = all). */
function scopeOf(command: KubectlCommand, env: Pick<KubectlEnv, 'currentNamespace'>): string {
  if (command.allNamespaces) return ''
  return command.namespace ?? env.currentNamespace
}

/** The argument being typed, if the input does not end with a space. */
function typing(command: KubectlCommand, index: number): string | undefined {
  if (command.pendingFlag || command.trailingSpace) return undefined
  return command.args.length === index + 1 ? command.args[index] : undefined
}

const startsWith = (text: string, prefix: string) => text.toLowerCase().startsWith(prefix.toLowerCase())

/**
 * The input with its last word replaced (or, after a space, a word
 * appended); a `--flag=value` word keeps its flag.
 */
export function withLastWord(input: string, word: string): string {
  const trimmed = input.replace(/\s+$/, '')
  if (/\s$/.test(input) || !trimmed) return `${trimmed}${trimmed ? ' ' : ''}${word}`
  return trimmed.replace(/(--[a-z-]+=)?\S*$/, (_match, flag: string | undefined) => `${flag ?? ''}${word}`)
}

export function kubectlNeeds<K extends KindInfo>(
  command: KubectlCommand,
  kinds: readonly K[],
  currentNamespace: string,
): KubectlNeeds<K> {
  const scope = scopeOf(command, { currentNamespace })
  const needs: KubectlNeeds<K> = { namespaces: command.pendingFlag === 'namespace' || command.verb === 'ns', clusters: command.verb === 'ctx' }
  if (command.verb === 'logs' || command.verb === 'exec') needs.pods = { namespace: scope }
  if (command.verb === 'get' || command.verb === 'describe') {
    const { kind: kindName } = command.verb === 'describe' ? splitKindName(command.args[0] ?? '') : { kind: command.args[0] ?? '' }
    const kind = findKind(kinds, kindName)
    const wantsNames =
      command.verb === 'describe' ? (command.args[0]?.includes('/') ?? false) || command.args.length > 1 || command.trailingSpace : command.args.length > 1 || (command.args.length === 1 && command.trailingSpace)
    // While a `-n` value is typed, objects are read for it too: a complete namespace name runs at once.
    const typedNamespace = command.pendingFlag !== 'namespace' || Boolean(command.namespace)
    if (kind && wantsNames && command.pendingFlag !== 'container' && typedNamespace) {
      needs.objects = { kind, namespace: kind.namespaced ? scope : '' }
    }
  }
  return needs
}

/** Rows for the verbs and aliases the input starts to name (when it is not a command yet). */
export function verbRows(input: string, aliases: readonly Alias[], t: KubectlEnv['t']): PaletteRow[] {
  const word = input.trim()
  if (!word || /\s/.test(word)) return []
  const verbs = VERBS.filter((verb) => verb.startsWith(word.toLowerCase())).map(
    (verb): PaletteRow => ({
      id: `verb:${verb}`,
      label: t(`palette.kubectl.usage.${verb}`),
      detail: t(`palette.kubectl.help.${verb}`),
      icon: 'terminal-cmd',
      complete: `${verb} `,
    }),
  )
  const matching = aliases
    .filter((alias) => alias.name.startsWith(word))
    .map(
      (alias): PaletteRow => ({
        id: `alias:${alias.name}`,
        label: `${alias.name} → ${alias.expansion}`,
        detail: t('palette.kubectl.alias'),
        icon: 'symbol-key',
        complete: `${alias.name} `,
      }),
    )
  return [...verbs, ...matching]
}

function infoRow(id: string, label: string, icon = 'info'): PaletteRow {
  return { id, label, icon }
}

/** The row shown while a command's data loads, or why it failed to. */
function pendingRow(env: KubectlEnv): PaletteRow {
  return env.loadError
    ? infoRow('error', env.t('palette.kubectl.loadFailed', { error: env.loadError }), 'error')
    : infoRow('loading', env.t('palette.kubectl.loading'), 'loading')
}

function namespaceRows(command: KubectlCommand, input: string, env: KubectlEnv, run?: (namespace: string) => void): PaletteRow[] {
  if (!env.namespaces) return [pendingRow(env)]
  const prefix = command.trailingSpace ? '' : (command.namespace ?? '')
  return env.namespaces
    .filter((ns) => startsWith(ns, prefix))
    .slice(0, MAX_ROWS)
    .map((ns) => ({
      id: `ns:${ns}`,
      label: ns,
      detail: env.t('palette.kubectl.namespaceDetail'),
      icon: 'folder',
      complete: `${withLastWord(input, ns)} `,
      run: run && (() => run(ns)),
    }))
}

/**
 * Whether the `-n` value being typed already names a namespace: the
 * command's own rows then show (Enter runs it) instead of completions.
 */
function namespaceComplete(command: KubectlCommand, env: KubectlEnv): boolean {
  return command.namespace !== undefined && (env.namespaces?.includes(command.namespace) ?? false)
}

function getRows(command: KubectlCommand, input: string, env: KubectlEnv): PaletteRow[] {
  const isDescribe = command.verb === 'describe'
  const first = command.args[0] ?? ''
  const split = isDescribe ? splitKindName(first) : { kind: first, name: command.args[1] }
  const kindTyping = isDescribe ? !first.includes('/') && command.args.length <= 1 && !command.trailingSpace : typing(command, 0) !== undefined
  const scope = scopeOf(command, env)

  // Choosing the kind.
  if (!first || (kindTyping && !findKind(env.kinds, split.kind)) || (kindTyping && isDescribe)) {
    const prefix = first
    return env.kinds
      .filter((k) => k.names.some((n) => startsWith(n, prefix)) || startsWith(k.kind, prefix))
      .map((k) => ({
        id: `kind:${k.kind}`,
        label: `${command.verb} ${k.names[0]}`,
        detail: [k.kind, ...k.names.slice(1)].join(', '),
        icon: k.icon ?? 'symbol-class',
        complete: isDescribe ? `${command.verb} ${k.names[0]}/` : `${command.verb} ${k.names[0]} `,
        run: isDescribe ? undefined : () => k.openList(k.namespaced && command.namespace ? command.namespace : undefined),
      }))
  }

  const kind = findKind(env.kinds, split.kind)
  if (!kind) return [infoRow('unknown', env.t('palette.kubectl.unknownKind', { kind: split.kind }), 'warning')]

  if (command.pendingFlag === 'namespace' && !namespaceComplete(command, env)) {
    return namespaceRows(command, input, env, isDescribe ? undefined : (ns) => kind.openList(kind.namespaced ? ns : undefined))
  }

  const rows: PaletteRow[] = []
  const name = split.name ?? (isDescribe ? command.args[1] : undefined)
  if (!isDescribe && name === undefined) {
    const where = kind.namespaced ? (command.namespace ?? (scope || null)) : null
    rows.push({
      id: 'open-list',
      label: where
        ? env.t('palette.kubectl.openListIn', { kind: kind.names[0], namespace: where })
        : env.t('palette.kubectl.openList', { kind: kind.names[0] }),
      icon: kind.icon ?? 'list-flat',
      run: () => kind.openList(kind.namespaced && command.namespace ? command.namespace : undefined),
    })
    if (!command.trailingSpace) return rows
  }

  if (!env.objects) return [...rows, pendingRow(env)]
  const prefix = name ?? ''
  const objects = env.objects.filter((o) => startsWith(o.name, prefix)).slice(0, MAX_ROWS)
  for (const object of objects) {
    const qualified = object.namespace ? `${object.namespace}/${object.name}` : object.name
    rows.push({
      id: `obj:${qualified}`,
      label: `${command.verb} ${kind.names[1] ?? kind.names[0]}/${object.name}`,
      detail: object.namespace ? env.t('palette.kubectl.inNamespace', { namespace: object.namespace }) : kind.kind,
      icon: kind.icon ?? 'symbol-field',
      complete: isDescribe
        ? `describe ${kind.names[1] ?? kind.names[0]}/${object.name}${object.namespace ? ` -n ${object.namespace}` : ''}`
        : `get ${kind.names[0]} ${object.name}${object.namespace ? ` -n ${object.namespace}` : ''}`,
      run: () => kind.openDetail(object),
    })
  }
  if (rows.length === 0) rows.push(infoRow('none', env.t('palette.kubectl.noObjects', { kind: kind.names[0] })))
  return rows
}

function podRows(command: KubectlCommand, input: string, env: KubectlEnv): PaletteRow[] {
  const isExec = command.verb === 'exec'
  if (command.pendingFlag === 'namespace' && !namespaceComplete(command, env)) return namespaceRows(command, input, env)
  if (!env.pods) return [pendingRow(env)]
  const prefix = command.args[0] ?? ''
  const exact = env.pods.filter((p) => p.name === prefix)
  const pods = (exact.length > 0 && command.trailingSpace ? exact : env.pods.filter((p) => startsWith(p.name, prefix))).slice(0, MAX_ROWS)

  if (command.pendingFlag === 'container') {
    const pod = exact[0]
    if (!pod) return [infoRow('pod-first', env.t('palette.kubectl.podFirst'))]
    const containerPrefix = command.trailingSpace ? '' : (command.container ?? '')
    return pod.containers
      .filter((c) => startsWith(c, containerPrefix))
      .map((c) => podRow(command, env, pod, c, isExec, `${withLastWord(input, c)} `))
  }

  const rows = pods.flatMap((pod) => {
    const container = command.container && pod.containers.includes(command.container) ? command.container : pod.containers[0]
    if (!container) return []
    const complete = `${command.verb} ${pod.name} -n ${pod.namespace}${command.container ? ` -c ${command.container}` : ''} `
    return [podRow(command, env, pod, container, isExec, complete)]
  })
  if (rows.length === 0) return [infoRow('none', env.t('palette.kubectl.noPods'))]
  return rows
}

function podRow(command: KubectlCommand, env: KubectlEnv, pod: PodInfo, container: string, isExec: boolean, complete: string): PaletteRow {
  const target = { namespace: pod.namespace, pod: pod.name, container }
  const reason = env.denied(isExec ? 'exec' : 'read')
  return {
    id: `pod:${pod.namespace}/${pod.name}/${container}`,
    label: `${command.verb} ${pod.namespace}/${pod.name}`,
    detail: env.t('palette.kubectl.containerDetail', { container }),
    icon: isExec ? 'terminal' : 'output',
    complete,
    disabledReason: reason ?? undefined,
    run: reason ? undefined : () => (isExec ? env.actions.openShell(target) : env.actions.showLogs(target)),
  }
}

function nsRows(command: KubectlCommand, input: string, env: KubectlEnv): PaletteRow[] {
  if (!env.namespaces) return [pendingRow(env)]
  const prefix = typing(command, 0) ?? (command.trailingSpace ? '' : (command.args[0] ?? ''))
  const all: PaletteRow = {
    id: 'ns:*',
    label: env.t('palette.kubectl.allNamespaces'),
    icon: 'folder-library',
    detail: env.currentNamespace === '' ? env.t('palette.kubectl.current') : undefined,
    run: () => env.actions.setNamespace(''),
  }
  const rows = env.namespaces
    .filter((ns) => startsWith(ns, prefix))
    .slice(0, MAX_ROWS)
    .map(
      (ns): PaletteRow => ({
        id: `ns:${ns}`,
        label: `ns ${ns}`,
        detail: ns === env.currentNamespace ? env.t('palette.kubectl.current') : undefined,
        icon: 'folder',
        complete: `${withLastWord(input, ns)}`,
        run: () => env.actions.setNamespace(ns),
      }),
    )
  return prefix ? rows : [all, ...rows]
}

function ctxRows(command: KubectlCommand, input: string, env: KubectlEnv): PaletteRow[] {
  if (!env.clusters) return [pendingRow(env)]
  const prefix = command.trailingSpace ? '' : (command.args[0] ?? '')
  const rows = env.clusters
    .filter((c) => startsWith(c.name, prefix))
    .map(
      (c): PaletteRow => ({
        id: `ctx:${c.name}`,
        label: `ctx ${c.name}`,
        detail: c.active ? env.t('palette.kubectl.current') : undefined,
        icon: 'server-environment',
        complete: withLastWord(input, c.name),
        disabledReason: c.disabledReason,
        run: c.active || c.disabledReason ? undefined : () => env.actions.switchCluster(c.name),
      }),
    )
  return rows.length > 0 ? rows : [infoRow('none', env.t('palette.kubectl.noClusters'))]
}

const READ_VERBS: Verb[] = ['get', 'describe', 'logs', 'ns']

export function kubectlRows(command: KubectlCommand, input: string, env: KubectlEnv): PaletteRow[] {
  const denied = READ_VERBS.includes(command.verb) || command.verb === 'exec' ? env.denied('read') : null
  if (denied) return [{ id: 'denied', label: env.t(`palette.kubectl.usage.${command.verb}`), icon: 'lock', disabledReason: denied }]
  switch (command.verb) {
    case 'get':
    case 'describe':
      return getRows(command, input, env)
    case 'logs':
    case 'exec':
      return podRows(command, input, env)
    case 'ns':
      return nsRows(command, input, env)
    case 'ctx':
      return ctxRows(command, input, env)
  }
}
