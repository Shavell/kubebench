#!/usr/bin/env node
// Verifies translation bundles are complete and consistent.
//
// Every directory named `locales` under src/ and examples/ holds one JSON
// file per language (en.json, tr.json, ...). The language set is defined by
// src/i18n/locales; every other locales directory must ship the same set.
// For each language the key set must equal the English one, values must be
// non-empty strings, and {{interpolation}} variables must match English.
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
    for (const [key, value] of reference) {
      if (!bundle.has(key)) {
        problems.push(`${file}: missing "${key}"`)
        continue
      }
      const translated = bundle.get(key)
      if (!translated.trim()) problems.push(`${file}: "${key}" is empty`)
      const want = [...vars(value)].sort().join(',')
      const got = [...vars(translated)].sort().join(',')
      if (want !== got) problems.push(`${file}: "${key}" interpolation {{${got}}} differs from ${REFERENCE} {{${want}}}`)
    }
    for (const key of bundle.keys()) {
      if (!reference.has(key)) problems.push(`${file}: extra key "${key}" (not in ${REFERENCE}.json)`)
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
