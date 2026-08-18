import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StatusTag from './StatusTag.vue'

const ElTagStub = {
  props: ['type'],
  template: '<span :data-type="type"><slot /></span>'
}

describe('StatusTag', () => {
  it('renders the configured label and semantic type', () => {
    const wrapper = mount(StatusTag, {
      props: {
        value: 'failed',
        map: { failed: { label: '失败', type: 'danger' } }
      },
      global: { stubs: { ElTag: ElTagStub } }
    })
    expect(wrapper.text()).toBe('失败')
    expect(wrapper.get('span').attributes('data-type')).toBe('danger')
  })

  it('falls back to the raw value for unknown dictionary entries', () => {
    const wrapper = mount(StatusTag, {
      props: { value: 'custom', map: {} },
      global: { stubs: { ElTag: ElTagStub } }
    })
    expect(wrapper.text()).toBe('custom')
  })
})
