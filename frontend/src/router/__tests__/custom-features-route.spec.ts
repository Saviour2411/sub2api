import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts'),
  'utf8'
)

describe('custom features admin route', () => {
  it('keeps the isolated admin page protected by admin authentication', () => {
    expect(routerSource).toContain("path: '/admin/custom-features'")
    expect(routerSource).toContain("component: () => import('@/views/admin/CustomFeaturesView.vue')")
    expect(routerSource).toMatch(/path: '\/admin\/custom-features'[\s\S]*?requiresAdmin: true/)
  })
})
