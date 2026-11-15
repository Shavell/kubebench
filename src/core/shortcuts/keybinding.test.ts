import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { isPlainKey, keyCaps, matchesKeybinding, parseKeybinding, type KeyStroke } from './keybinding.ts'

const stroke = (key: string, code: string, mods: Partial<KeyStroke> = {}): KeyStroke => ({
  key,
  code,
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  ...mods,
})

describe('keybinding', () => {
  it('parses modifiers and keys', () => {
    assert.deepEqual(parseKeybinding('Ctrl+Shift+P'), { ctrl: true, shift: true, alt: false, key: 'P' })
    assert.deepEqual(parseKeybinding('Ctrl++'), { ctrl: true, shift: false, alt: false, key: '+' })
    assert.deepEqual(parseKeybinding('?'), { ctrl: false, shift: false, alt: false, key: '?' })
  })

  it('matches letters by physical key, with Ctrl or Cmd', () => {
    assert.ok(matchesKeybinding(stroke('b', 'KeyB', { ctrlKey: true }), 'Ctrl+B'))
    assert.ok(matchesKeybinding(stroke('b', 'KeyB', { metaKey: true }), 'Ctrl+B'))
    // macOS: Option+N types a dead key, the code still names N.
    assert.ok(matchesKeybinding(stroke('˜', 'KeyN', { altKey: true }), 'Alt+N'))
    assert.ok(!matchesKeybinding(stroke('b', 'KeyB'), 'Ctrl+B'))
    assert.ok(!matchesKeybinding(stroke('B', 'KeyB', { ctrlKey: true, shiftKey: true }), 'Ctrl+B'))
    assert.ok(!matchesKeybinding(stroke('b', 'KeyB', { ctrlKey: true, altKey: true }), 'Ctrl+B'))
  })

  it('matches characters by what they type, whatever Shift does', () => {
    assert.ok(matchesKeybinding(stroke('?', 'Slash', { shiftKey: true }), '?'))
    assert.ok(matchesKeybinding(stroke('?', 'Minus', { shiftKey: true }), '?'))
    assert.ok(matchesKeybinding(stroke('/', 'Slash'), '/'))
    assert.ok(matchesKeybinding(stroke(',', 'Comma', { ctrlKey: true }), 'Ctrl+,'))
    assert.ok(!matchesKeybinding(stroke('/', 'Slash', { ctrlKey: true }), '/'))
  })

  it('matches named keys with exact Shift', () => {
    assert.ok(matchesKeybinding(stroke('PageDown', 'PageDown', { altKey: true }), 'Alt+PageDown'))
    assert.ok(!matchesKeybinding(stroke('PageDown', 'PageDown', { altKey: true, shiftKey: true }), 'Alt+PageDown'))
    assert.ok(matchesKeybinding(stroke('F10', 'F10', { shiftKey: true }), 'Shift+F10'))
  })

  it('matches the backquote key physically', () => {
    assert.ok(matchesKeybinding(stroke('"', 'Backquote', { ctrlKey: true }), 'Ctrl+`'))
  })

  it('tells plain keys apart', () => {
    assert.ok(isPlainKey('?'))
    assert.ok(isPlainKey('Shift+F10'))
    assert.ok(!isPlainKey('Alt+W'))
  })

  it('renders key caps per platform', () => {
    assert.deepEqual(keyCaps('Ctrl+Shift+P', false), ['Ctrl', 'Shift', 'P'])
    assert.deepEqual(keyCaps('Ctrl+Shift+P', true), ['⌘', '⇧', 'P'])
    assert.deepEqual(keyCaps('Alt+PageDown', false), ['Alt', 'PageDown'])
  })
})
