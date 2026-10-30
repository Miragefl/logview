// portal 容器判定:naive-ui 的下拉菜单/弹窗/抽屉默认 portal 渲染到 document.body,
// 不在任何组件 DOM 子树内——外部点击收起判定必须放行这些容器内的点击,
// 否则点 NSelect 选项/NModal 密码框会被误判"点外面"导致面板卸载(验收反馈①)。

/** PORTAL_CLASSES naive-ui 系 portal 挂载容器类名。 */
const PORTAL_CLASSES = [
  'v-binder-follower-container', // NPopover/NSelect/NDropdown 菜单
  'n-modal-container', // NModal
  'n-drawer-container', // NDrawer
]

/** isPortalNode 节点(含祖先链)是否落在任一 portal 容器内;null 安全。 */
export function isPortalNode(node: Node | null): boolean {
  let cur: Node | null = node
  while (cur) {
    const cl = (cur as HTMLElement).classList
    if (cl && PORTAL_CLASSES.some((c) => cl.contains(c))) {
      return true
    }
    cur = cur.parentNode
  }
  return false
}
