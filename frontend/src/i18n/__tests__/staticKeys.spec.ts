import fs from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleNode = Record<string, unknown>

function flattenLocaleKeys(node: unknown, prefix = '', out = new Set<string>()): Set<string> {
  if (node && typeof node === 'object' && !Array.isArray(node)) {
    for (const [key, value] of Object.entries(node as LocaleNode)) {
      flattenLocaleKeys(value, prefix ? `${prefix}.${key}` : key, out)
    }
    return out
  }

  if (prefix) out.add(prefix)
  return out
}

function walkSourceFiles(dir: string, out: string[] = []): string[] {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (['node_modules', 'dist', '.git'].includes(entry.name)) continue
    const filePath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      walkSourceFiles(filePath, out)
    } else if (/\.(vue|ts|tsx|js)$/.test(entry.name) && !filePath.includes(`${path.sep}i18n${path.sep}locales${path.sep}`)) {
      out.push(filePath)
    }
  }
  return out
}

function collectStaticI18nKeys(): Map<string, Set<string>> {
  const root = path.resolve(__dirname, '..', '..')
  const files = walkSourceFiles(root)
  const keys = new Map<string, Set<string>>()
  const patterns = [
    /\b(?:t|\$t)\(\s*['"]([A-Za-z0-9_.-]+)['"]/g,
    /\b(?:titleKey|descriptionKey|labelKey)\s*:\s*['"]([A-Za-z0-9_.-]+)['"]/g,
  ]

  for (const file of files) {
    const source = fs.readFileSync(file, 'utf8')
    for (const pattern of patterns) {
      let match: RegExpExecArray | null
      while ((match = pattern.exec(source))) {
        const key = match[1]
        if (!key || key.endsWith('.') || key === 'label') continue
        if (!keys.has(key)) keys.set(key, new Set())
        keys.get(key)?.add(path.relative(root, file))
      }
    }
  }

  return keys
}

describe('static i18n keys', () => {
  it.each([
    ['zh', flattenLocaleKeys(zh)],
    ['en', flattenLocaleKeys(en)],
  ])('has all statically referenced keys in %s', (_locale, availableKeys) => {
    const usedKeys = collectStaticI18nKeys()
    const missing = [...usedKeys.keys()]
      .filter((key) => !availableKeys.has(key))
      .sort()
      .map((key) => `${key} (${[...(usedKeys.get(key) ?? [])].slice(0, 3).join(', ')})`)

    expect(missing).toEqual([])
  })
})
