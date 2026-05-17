import { afterEach, describe, expect, it, vi } from 'vitest'

describe('i18n 初始化', () => {
  const originalLocalStorageDescriptor = Object.getOwnPropertyDescriptor(window, 'localStorage')

  afterEach(() => {
    if (originalLocalStorageDescriptor) {
      Object.defineProperty(window, 'localStorage', originalLocalStorageDescriptor)
    }
    vi.resetModules()
  })

  it('localStorage 被浏览器阻止时仍可导入模块', async () => {
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      get() {
        throw new DOMException('Access denied', 'SecurityError')
      }
    })

    await expect(import('@/i18n')).resolves.toHaveProperty('i18n')
  })
})
