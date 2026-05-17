import { afterEach, describe, expect, it, vi } from 'vitest'
import { safeLocalStorage, safeSessionStorage } from '@/utils/browserStorage'

describe('browserStorage', () => {
  const originalLocalStorageDescriptor = Object.getOwnPropertyDescriptor(window, 'localStorage')

  afterEach(() => {
    if (originalLocalStorageDescriptor) {
      Object.defineProperty(window, 'localStorage', originalLocalStorageDescriptor)
    }
    localStorage.clear()
    sessionStorage.clear()
  })

  it('localStorage 属性访问被阻止时安全降级', () => {
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      get() {
        throw new DOMException('Access denied', 'SecurityError')
      }
    })

    expect(safeLocalStorage.getItem('auth_token')).toBeNull()
    expect(safeLocalStorage.setItem('auth_token', 'token')).toBe(false)
    expect(safeLocalStorage.removeItem('auth_token')).toBe(false)
  })

  it('Storage 方法抛错时不向外抛异常', () => {
    const getItemSpy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new DOMException('Blocked', 'SecurityError')
    })
    const setItemSpy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('Blocked', 'SecurityError')
    })
    const removeItemSpy = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new DOMException('Blocked', 'SecurityError')
    })

    try {
      expect(safeSessionStorage.getItem('register_data')).toBeNull()
      expect(safeSessionStorage.setItem('register_data', '{}')).toBe(false)
      expect(safeSessionStorage.removeItem('register_data')).toBe(false)
    } finally {
      getItemSpy.mockRestore()
      setItemSpy.mockRestore()
      removeItemSpy.mockRestore()
    }
  })
})
