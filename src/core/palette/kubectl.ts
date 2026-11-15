/**
 * kubectl-style palette commands (pure, unit tested in kubectl.test.ts):
 *
 *   get <kind> [name] [-n ns | -A]   describe <kind>/<name> [-n ns]
 *   logs <pod> [-n ns] [-c container] exec <pod> [-n ns] [-c container]
 *   ns [name]                          ctx [cluster]
 *
 * An optional leading `kubectl` (or `k`) is ignored, and user-defined
 * aliases expand first: alias `kgp` = `get pods` makes `kgp -n shop` mean
 * `get pods -n shop`.
 */
export const VERBS = ['get', 'describe', 'logs', 'exec', 'ns', 'ctx'] as const
export type Verb = (typeof VERBS)[number]

export interface KubectlCommand {
  verb: Verb
  /** Positional arguments after the verb. */
  args: string[]
  /** `-n` / `--namespace`. */
  namespace?: string
  /** `-A` / `--all-namespaces`. */
  allNamespaces: boolean
  /** `-c` / `--container`. */
  container?: string
  /**
   * The flag whose value is being typed (input ends with `-n ` or `-n sh`),
   * so completion can offer its values.
   */
  pendingFlag?: 'namespace' | 'container'
  /** The input ends with whitespace: the next word has not been started. */
  trailingSpace: boolean
}

export interface Alias {
  name: string
  expansion: string
}

const isVerb = (word: string): word is Verb => (VERBS as readonly string[]).includes(word)

export function tokenize(input: string): string[] {
  return input.trim().split(/\s+/).filter(Boolean)
}

/**
 * Replaces a leading alias by its expansion (one level: an expansion is
 * never expanded again), keeping the rest of the input.
 */
export function expandAlias(input: string, aliases: readonly Alias[]): string {
  const match = /^\s*(\S+)(.*)$/s.exec(input)
  if (!match) return input
  const alias = aliases.find((a) => a.name === match[1])
  return alias ? `${alias.expansion.trim()}${match[2]}` : input
}

const FLAGS: Record<string, 'namespace' | 'container'> = {
  '-n': 'namespace',
  '--namespace': 'namespace',
  '-c': 'container',
  '--container': 'container',
}

/** Parses a kubectl-style command, or returns null when the input is not one. */
export function parseKubectl(input: string): KubectlCommand | null {
  let words = tokenize(input)
  if (words[0] === 'kubectl' || words[0] === 'k') words = words.slice(1)
  const [verb, ...rest] = words
  if (!verb || !isVerb(verb)) return null

  const command: KubectlCommand = { verb, args: [], allNamespaces: false, trailingSpace: /\s$/.test(input) }
  for (let i = 0; i < rest.length; i++) {
    const word = rest[i]
    const [flag, inline] = word.startsWith('--') && word.includes('=') ? word.split(/=(.*)/s, 2) : [word, undefined]
    const target = FLAGS[flag]
    if (target) {
      const value = inline ?? rest[i + 1]
      if (inline === undefined) i++
      if (value === undefined) {
        command.pendingFlag = target
        continue
      }
      command[target] = value
      // A value still being typed (no space after it) keeps completing.
      if (i === rest.length - 1 && !command.trailingSpace) command.pendingFlag = target
    } else if (word === '-A' || word === '--all-namespaces') {
      command.allNamespaces = true
    } else {
      command.args.push(word)
    }
  }
  return command
}

/** `pod/web-1` → kind and name; a bare `pod` → kind only. */
export function splitKindName(arg: string): { kind: string; name?: string } {
  const slash = arg.indexOf('/')
  return slash < 0 ? { kind: arg } : { kind: arg.slice(0, slash), name: arg.slice(slash + 1) }
}

/** Why an alias cannot be saved (an i18n key in the platform namespace), or null. */
export function aliasError(alias: Alias, others: readonly Alias[]): string | null {
  const name = alias.name.trim()
  if (!name) return 'palette.aliases.errors.nameRequired'
  if (/\s/.test(name)) return 'palette.aliases.errors.nameSpaces'
  if (isVerb(name) || name === 'kubectl' || name === 'k') return 'palette.aliases.errors.nameReserved'
  if (others.some((a) => a.name === name)) return 'palette.aliases.errors.nameTaken'
  if (!alias.expansion.trim()) return 'palette.aliases.errors.expansionRequired'
  if (!parseKubectl(alias.expansion)) return 'palette.aliases.errors.expansionInvalid'
  return null
}
