type StorageKind = 'local' | 'session'

function getStorage(kind: StorageKind): Storage | null {
  if (typeof window === 'undefined') {
    return null
  }

  try {
    return kind === 'local' ? window.localStorage : window.sessionStorage
  } catch {
    return null
  }
}

export function safeStorageGetItem(kind: StorageKind, key: string): string | null {
  const storage = getStorage(kind)
  if (!storage) {
    return null
  }

  try {
    return storage.getItem(key)
  } catch {
    return null
  }
}

export function safeStorageSetItem(kind: StorageKind, key: string, value: string): boolean {
  const storage = getStorage(kind)
  if (!storage) {
    return false
  }

  try {
    storage.setItem(key, value)
    return true
  } catch {
    return false
  }
}

export function safeStorageRemoveItem(kind: StorageKind, key: string): boolean {
  const storage = getStorage(kind)
  if (!storage) {
    return false
  }

  try {
    storage.removeItem(key)
    return true
  } catch {
    return false
  }
}

export const safeLocalStorage = {
  getItem: (key: string) => safeStorageGetItem('local', key),
  setItem: (key: string, value: string) => safeStorageSetItem('local', key, value),
  removeItem: (key: string) => safeStorageRemoveItem('local', key),
}

export const safeSessionStorage = {
  getItem: (key: string) => safeStorageGetItem('session', key),
  setItem: (key: string, value: string) => safeStorageSetItem('session', key, value),
  removeItem: (key: string) => safeStorageRemoveItem('session', key),
}
