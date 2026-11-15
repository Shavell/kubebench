import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { languageMatcher, regionalLocale } from './languageMatch.ts'

describe('languageMatcher', () => {
  const match = languageMatcher(['en', 'de', 'tr', 'pt-BR'])

  it('matches a regional tag by its base language', () => {
    assert.equal(match('de-DE'), 'de')
    assert.equal(match('de-AT'), 'de')
    assert.equal(match('tr-TR'), 'tr')
    assert.equal(match('en-GB'), 'en')
  })

  it('prefers an exact match and ignores case', () => {
    assert.equal(match('DE'), 'de')
    assert.equal(match('pt-br'), 'pt-BR')
  })

  it('returns undefined for unsupported languages', () => {
    assert.equal(match('fr-FR'), undefined)
    assert.equal(match('pt-PT'), undefined)
  })
})

describe('regionalLocale', () => {
  const match = languageMatcher(['en', 'de', 'tr'])

  it('uses the browser region of the UI language', () => {
    assert.equal(regionalLocale('de', ['en-US', 'de-AT', 'de-DE'], match), 'de-AT')
    assert.equal(regionalLocale('en', ['en-GB'], match), 'en-GB')
  })

  it('falls back to the UI language when the browser does not list it', () => {
    assert.equal(regionalLocale('de', ['en-US', 'tr-TR'], match), 'de')
    assert.equal(regionalLocale('tr', [], match), 'tr')
  })
})
