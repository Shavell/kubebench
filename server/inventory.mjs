import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'

const DIR = join(homedir(), '.kubebench')
const FILE = join(DIR, 'clusters.json')

/**
 * Persistent cluster inventory: named kubeconfig paths (or bare API
 * addresses) plus which one is active. Stores paths, never credentials.
 */
export function loadInventory() {
  if (!existsSync(FILE)) return { active: null, clusters: [] }
  try {
    return JSON.parse(readFileSync(FILE, 'utf8'))
  } catch {
    return { active: null, clusters: [] }
  }
}

export function saveInventory(inventory) {
  mkdirSync(DIR, { recursive: true })
  writeFileSync(FILE, JSON.stringify(inventory, null, 2))
}

export function inventoryPath() {
  return FILE
}
