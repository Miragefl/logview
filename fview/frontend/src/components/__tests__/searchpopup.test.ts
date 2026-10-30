// SearchPopup 交互回归(Final Review #5):Tab 轮换分区后输入框必须重新聚焦,
// 否则键盘流断裂(Esc/j 落回 App 全局键位)。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

vi.mock('../../lib/backend', () => ({
  matchLines: vi.fn().mockResolvedValue([0]),
  parseQuery: vi.fn().mockResolvedValue({ parseError: '', highlightKeywords: [], timeRangeHint: '' }),
  usageKeywords: vi.fn().mockResolvedValue([]),
  bumpKeyword: vi.fn().mockResolvedValue(undefined),
}))

import SearchPopup from '../logview/SearchPopup.vue'
import { useSearchStore } from '../../stores/searchStore'

describe('SearchPopup', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('Tab 轮换分区后输入框保持聚焦(Final #5)', async () => {
    const search = useSearchStore()
    const w = mount(SearchPopup, { global: { plugins: [naive] } })
    search.open(0) // mount 后再开,模拟真实时序(show false→true 触发 watch 聚焦)
    await new Promise((r) => setTimeout(r, 80))

    const input = document.body.querySelector('input')
    expect(input).toBeTruthy()
    expect(document.activeElement).toBe(input)

    // Tab 键轮换到高亮分区 → 新输入框应自动聚焦
    input!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    await new Promise((r) => setTimeout(r, 80))
    const input2 = document.body.querySelector('input')
    expect(input2).toBeTruthy()
    expect(document.activeElement).toBe(input2)
  })
})
