// isPortalNode 判定测试:naive-ui portal 到 body 的容器(NSelect 菜单/NModal/NDrawer)
// 内的点击不算"面板外部"——SidebarRail 收起判定的 portal 感知(验收反馈①)。
import { describe, expect, it } from 'vitest'
import { isPortalNode } from './portal'

/** mkStub 手搓节点桩:classList.contains + parentNode,结构对齐 isPortalNode 的访问面。 */
function mkNode(classes: string[], parent: unknown = null): Node {
  return {
    classList: { contains: (k: string) => classes.includes(k) },
    parentNode: parent,
  } as unknown as Node
}

describe('isPortalNode', () => {
  it('祖先链含 v-binder-follower-container(NSelect 下拉菜单)→ true', () => {
    const menu = mkNode(['n-select-menu'], mkNode(['v-binder-follower-container'], mkNode(['body-ish'])))
    expect(isPortalNode(menu)).toBe(true)
  })

  it('祖先链含 n-modal-container(NModal 内的密码框)→ true', () => {
    const input = mkNode(['n-input'], mkNode(['n-modal'], mkNode(['n-modal-container'])))
    expect(isPortalNode(input)).toBe(true)
  })

  it('普通 DOM 链(无 portal 容器)→ false', () => {
    const row = mkNode(['res-item'], mkNode(['panel'], mkNode(['flyout'])))
    expect(isPortalNode(row)).toBe(false)
  })

  it('null → false', () => {
    expect(isPortalNode(null)).toBe(false)
  })
})
