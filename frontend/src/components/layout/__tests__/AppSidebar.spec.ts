import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const homeViewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../views/HomeView.vue')
const homeViewSource = readFileSync(homeViewPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('custom feature navigation entries', () => {
  it('keeps the independent custom feature admin entry', () => {
    expect(componentSource).toContain("path: '/admin/custom-features'")
    expect(componentSource).toContain("label: t('nav.customFeatures')")
  })

  it('wires model marketplace and daily check-in to their feature flags', () => {
    expect(componentSource).toContain('makeSidebarFlag(FeatureFlags.modelMarketplace)')
    expect(componentSource).toContain('makeSidebarFlag(FeatureFlags.dailyCheckin)')
    expect(componentSource).toContain("path: '/models', label: t('nav.modelMarketplace')")
    expect(componentSource).toContain("path: '/daily-checkin', label: t('nav.dailyCheckin')")
  })

  it('keeps a public model marketplace entry in the header and hero', () => {
    expect(homeViewSource).toContain('isFeatureFlagEnabled(FeatureFlags.modelMarketplace)')
    expect(homeViewSource.match(/to="\/models"/g)).toHaveLength(2)
    expect(homeViewSource.match(/v-if="modelMarketplaceEnabled"/g)).toHaveLength(2)
  })
})
