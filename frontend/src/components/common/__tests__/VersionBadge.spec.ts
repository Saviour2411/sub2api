import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import VersionBadge from '../VersionBadge.vue'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post }, default: { get, post } }))

describe('二开版本只读展示', () => {
  afterEach(() => vi.clearAllMocks())

  it('只显示当前版本，点击不弹出菜单且不发起请求', async () => {
    const wrapper = mount(VersionBadge, { props: { version: '0.1.241' } })
    expect(wrapper.text()).toBe('v0.1.241')
    await wrapper.get('span').trigger('click')
    await flushPromises()
    expect(wrapper.find('button, a, [role="dialog"]').exists()).toBe(false)
    expect(get).not.toHaveBeenCalled()
    expect(post).not.toHaveBeenCalled()
    await wrapper.setProps({ version: '0.1.242' })
    expect(wrapper.text()).toBe('v0.1.242')
    wrapper.unmount()
  })

  it('公共设置还没有版本时不显示虚构版本', () => {
    const wrapper = mount(VersionBadge)
    expect(wrapper.text()).toBe('')
    expect(get).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
