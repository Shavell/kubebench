#!/usr/bin/env node
// Verifies translation bundles are complete and consistent.
//
// Every directory named `locales` under src/ and examples/ holds one JSON
// file per language (en.json, tr.json, ...). The language set is defined by
// src/i18n/locales; every other locales directory must ship the same set.
// For each language the key set must equal the English one, values must be
// non-empty strings, and {{interpolation}} variables must match English.
// Plural keys (i18next's `<key>_one`, `<key>_other`, ...) are compared by
// their base: a language must have exactly the plural categories its CLDR
// rules use (Intl.PluralRules: one/other for English, one/few/many/other
// for Polish, ...), whatever categories English needs.
//
// Usage: node scripts/check-i18n.mjs        (exit code 1 on problems)

import { readdirSync, readFileSync, statSync, existsSync } from 'node:fs'
import { join, relative, basename } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const REFERENCE = 'en'
const CORE_DIR = join(root, 'src', 'i18n', 'locales')
const SEARCH_ROOTS = ['src', 'examples'].map((d) => join(root, d)).filter((d) => existsSync(d))

function findLocaleDirs(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    if (entry === 'node_modules' || entry === 'dist' || entry.startsWith('.')) continue
    const full = join(dir, entry)
    if (!statSync(full).isDirectory()) continue
    if (entry === 'locales') out.push(full)
    else findLocaleDirs(full, out)
  }
  return out
}

function languagesIn(dir) {
  return readdirSync(dir)
    .filter((f) => f.endsWith('.json'))
    .map((f) => basename(f, '.json'))
    .sort()
}

function flatten(obj, prefix, out, problems, file) {
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      flatten(value, path, out, problems, file)
    } else if (typeof value === 'string') {
      out.set(path, value)
    } else {
      problems.push(`${file}: "${path}" must be a string or object (got ${Array.isArray(value) ? 'array' : typeof value})`)
    }
  }
  return out
}

function load(file, problems) {
  try {
    return flatten(JSON.parse(readFileSync(file, 'utf8')), '', new Map(), problems, file)
  } catch (err) {
    problems.push(`${file}: invalid JSON (${err.message})`)
    return new Map()
  }
}

const vars = (s) => new Set([...s.matchAll(/\{\{\s*([\w.]+)[^}]*\}\}/g)].map((m) => m[1]))

const PLURAL_CATEGORIES = ['zero', 'one', 'two', 'few', 'many', 'other']
const pluralSplit = (key) => {
  const at = key.lastIndexOf('_')
  return at > 0 && PLURAL_CATEGORIES.includes(key.slice(at + 1)) ? [key.slice(0, at), key.slice(at + 1)] : null
}

/**
 * The keys a language must have for the reference bundle: plural keys are
 * expanded to the language's categories, each mapped to the reference key
 * whose interpolation it must match (the same category, else `_other`).
 */
function expectedKeys(reference, lang) {
  const categories = new Intl.PluralRules(lang).resolvedOptions().pluralCategories
  const plurals = new Set()
  for (const key of reference.keys()) {
    const split = pluralSplit(key)
    if (split && reference.has(`${split[0]}_other`)) plurals.add(split[0])
  }
  const expected = new Map()
  for (const key of reference.keys()) {
    const split = pluralSplit(key)
    if (!split || !plurals.has(split[0])) expected.set(key, key)
  }
  for (const base of plurals) {
    for (const category of categories) {
      const key = `${base}_${category}`
      expected.set(key, reference.has(key) ? key : `${base}_other`)
    }
  }
  return expected
}

const expectedLanguages = languagesIn(CORE_DIR)
if (!expectedLanguages.includes(REFERENCE)) {
  console.error(`check-i18n: ${relative(root, CORE_DIR)} has no ${REFERENCE}.json`)
  process.exit(1)
}

const problems = []
let keyCount = 0
const dirs = SEARCH_ROOTS.flatMap((d) => findLocaleDirs(d))

for (const dir of dirs) {
  const rel = relative(root, dir)
  const langs = languagesIn(dir)
  for (const lang of expectedLanguages) {
    if (!langs.includes(lang)) problems.push(`${rel}: missing ${lang}.json`)
  }
  for (const lang of langs) {
    if (!expectedLanguages.includes(lang)) {
      problems.push(`${rel}: ${lang}.json is not a supported language (add it to src/i18n/locales first)`)
    }
  }
  if (!langs.includes(REFERENCE)) continue

  const reference = load(join(dir, `${REFERENCE}.json`), problems)
  keyCount += reference.size
  for (const [key, value] of reference) {
    if (!value.trim()) problems.push(`${rel}/${REFERENCE}.json: "${key}" is empty`)
  }
  for (const lang of langs.filter((l) => l !== REFERENCE)) {
    const file = `${rel}/${lang}.json`
    const bundle = load(join(dir, `${lang}.json`), problems)
    const expected = expectedKeys(reference, lang)
    for (const [key, referenceKey] of expected) {
      if (!bundle.has(key)) {
        problems.push(`${file}: missing "${key}"`)
        continue
      }
      const translated = bundle.get(key)
      if (!translated.trim()) problems.push(`${file}: "${key}" is empty`)
      const want = [...vars(reference.get(referenceKey))].sort().join(',')
      const got = [...vars(translated)].sort().join(',')
      if (want !== got) problems.push(`${file}: "${key}" interpolation {{${got}}} differs from ${REFERENCE} {{${want}}}`)
    }
    for (const key of bundle.keys()) {
      if (expected.has(key)) continue
      const split = pluralSplit(key)
      problems.push(
        split && expected.has(`${split[0]}_other`)
          ? `${file}: extra key "${key}" (${lang} has no "${split[1]}" plural form)`
          : `${file}: extra key "${key}" (not in ${REFERENCE}.json)`,
      )
    }
  }
}

if (problems.length) {
  console.error(`check-i18n: ${problems.length} problem(s)\n`)
  for (const p of problems) console.error(`  - ${p}`)
  process.exit(1)
}
console.log(
  `check-i18n: OK — ${dirs.length} locale dir(s), ${keyCount} key(s), languages: ${expectedLanguages.join(', ')}`,
)
