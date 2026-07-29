import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfilePasskeyCard from '@/components/user/profile/ProfilePasskeyCard.vue'

const {
  listPasskeysMock,
  showErrorMock
} = vi.hoisted(() => ({
  listPasskeysMock: vi.fn(),
  showErrorMock: vi.fn()
}))

vi.mock('@/api', () => ({
  passkeyAPI: {
    isSupported: () => true,
    list: listPasskeysMock,
    register: vi.fn(),
    rename: vi.fn(),
    remove: vi.fn()
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: vi.fn()
  })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

function mountCard(enabled: boolean) {
  return mount(ProfilePasskeyCard, {
    props: {
      enabled
    },
    global: {
      stubs: {
        Icon: true
      }
    }
  })
}

describe('ProfilePasskeyCard', () => {
  beforeEach(() => {
    listPasskeysMock.mockReset()
    showErrorMock.mockReset()
  })

  it('功能禁用时不请求 Passkey 凭据列表', async () => {
    mountCard(false)
    await flushPromises()

    expect(listPasskeysMock).not.toHaveBeenCalled()
    expect(showErrorMock).not.toHaveBeenCalled()
  })

  it('设置变更竞态返回 PASSKEY_DISABLED 时保持静默', async () => {
    listPasskeysMock.mockRejectedValue({
      code: 403,
      reason: 'PASSKEY_DISABLED'
    })

    mountCard(true)
    await flushPromises()

    expect(listPasskeysMock).toHaveBeenCalledTimes(1)
    expect(showErrorMock).not.toHaveBeenCalled()
  })

  it('其他加载错误仍显示失败提示', async () => {
    listPasskeysMock.mockRejectedValue({
      code: 500,
      reason: 'INTERNAL_ERROR'
    })

    mountCard(true)
    await flushPromises()

    expect(showErrorMock).toHaveBeenCalledWith('profile.passkey.loadFailed')
  })
})
