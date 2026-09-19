import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ConfirmDialog from '../ConfirmDialog.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('确认窗口身份信息', () => {
  it('长邮箱消息可换行，确认与取消行为保持不变', async () => {
    const message = `重置 ${'a'.repeat(64)}@example.test（ID 1054）的查询链接？`
    const wrapper = mount(ConfirmDialog, {
      props: { show: true, title: '重置链接', message },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } }
    })
    expect(wrapper.get('p').text()).toBe(message)
    expect(wrapper.get('p').classes()).toContain('break-words')
    const buttons = wrapper.findAll('button')
    await buttons[0].trigger('click')
    expect(wrapper.emitted('cancel')).toHaveLength(1)
    await buttons[1].trigger('click')
    expect(wrapper.emitted('confirm')).toHaveLength(1)
    wrapper.unmount()
  })
})
