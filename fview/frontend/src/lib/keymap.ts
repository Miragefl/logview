// vim 键位机(验收反馈④):纯函数按键 → 动作,以 TUI internal/tui/app.go
// handleNormalKeys 为唯一准绳。级别档位 = 大写 E/W/I/D/A;小写 e/w/d 释放给
// 折叠/换行/详情。z 序列(zt/zz/zb)走 pendingZ 状态机,状态由调用方持有。
export type KeyAction =
  | 'cursorDown'
  | 'cursorUp'
  | 'gotoTop'
  | 'gotoBottom'
  | 'viewportTop'
  | 'viewportMid'
  | 'viewportBottom'
  | 'anchorTop'
  | 'anchorMid'
  | 'anchorBottom'
  | 'halfDown'
  | 'halfUp'
  | 'pageDown'
  | 'pageUp'
  | 'nextHit'
  | 'prevHit'
  | 'search'
  | 'highlight'
  | 'hide'
  | 'level'
  | 'fold'
  | 'detail'
  | 'wrap'
  | 'visual'
  | 'visualLine'
  | 'yank'
  | 'copySelection'
  | 'bookmark'
  | 'gotoBookmarks'
  | 'fields'
  | 'lineNumbers'
  | 'help'
  | 'clear'
  | 'escape'
  | 'sources'
  | 'settings'
  | 'activate'
  | 'closeTab'
  | 'tabSwitch'
  | 'zoomIn'
  | 'zoomOut'
  | 'navDown'
  | 'navUp'
  | 'pending'
  | 'none'

export interface KeyEventLike {
  key: string
  ctrlKey: boolean
  /** cmd(macOS meta)仅白名单键接管,见 META_OK_KEYS */
  metaKey?: boolean
}

export interface KeyState {
  pendingZ: boolean
}

export interface KeyResult {
  action: KeyAction
  /** level 档位 arg ∈ E|W|I|D|A */
  arg?: string
}

/** SIMPLE_MAP 无状态单键映射(z 序列与 ctrl 组合另查)。 */
const SIMPLE_MAP: Record<string, KeyResult> = {
  j: { action: 'cursorDown' },
  k: { action: 'cursorUp' },
  g: { action: 'gotoTop' },
  G: { action: 'gotoBottom' },
  H: { action: 'viewportTop' },
  M: { action: 'viewportMid' },
  L: { action: 'viewportBottom' },
  PageDown: { action: 'pageDown' },
  PageUp: { action: 'pageUp' },
  n: { action: 'nextHit' },
  N: { action: 'prevHit' },
  '/': { action: 'search' },
  f: { action: 'search' },
  h: { action: 'highlight' },
  x: { action: 'hide' },
  E: { action: 'level', arg: 'E' },
  W: { action: 'level', arg: 'W' },
  I: { action: 'level', arg: 'I' },
  D: { action: 'level', arg: 'D' },
  A: { action: 'level', arg: 'A' },
  e: { action: 'fold' },
  d: { action: 'detail' },
  w: { action: 'wrap' },
  v: { action: 'visual' },
  V: { action: 'visualLine' },
  y: { action: 'yank' },
  m: { action: 'bookmark' },
  "'": { action: 'gotoBookmarks' },
  F: { action: 'fields' },
  '#': { action: 'lineNumbers' },
  '?': { action: 'help' },
  C: { action: 'clear' },
  q: { action: 'sources' },
  ',': { action: 'settings' },
  Enter: { action: 'activate' },
  Escape: { action: 'escape' },
}

const CTRL_MAP: Record<string, KeyResult> = {
  d: { action: 'halfDown' },
  u: { action: 'halfUp' },
  f: { action: 'pageDown' },
  b: { action: 'pageUp' },
  w: { action: 'closeTab' },
  j: { action: 'navDown' },
  k: { action: 'navUp' },
  '=': { action: 'zoomIn' },
  '+': { action: 'zoomIn' },
  '-': { action: 'zoomOut' },
  ',': { action: 'settings' },
  c: { action: 'copySelection' },
}

/** META_OK_KEYS cmd(macOS)接管白名单:字号/设置/tab 数字(第七轮验收①③)/复制(第九轮④,
 *  ⌘C 有选区时拦截、无选区由 App 放行系统);其余仅 ctrl——cmd+w 走 mac 原生菜单
 *  (fview/menu.go 注册 ⌘W,事件路由 shortcut:closeTab 由 App.vue 订阅关 tab),
 *  keymap 不认,防漏进 vim 单键。 */
const META_OK_KEYS = new Set(['=', '+', '-', ',', 'c', '1', '2', '3', '4', '5', '6', '7', '8', '9'])

const Z_MAP: Record<string, KeyResult> = {
  t: { action: 'anchorTop' },
  z: { action: 'anchorMid' },
  b: { action: 'anchorBottom' },
}

/** resolveKey 按键 → 动作;st.pendingZ 由本函数维护(z 序列)。
 *  ctrl+1..9 切 tab(浏览器惯例,第四轮验收③全键盘);
 *  cmd 系白名单键(字号/设置/tab 数字)与 ctrl 等价,其余 cmd 交给系统。 */
export function resolveKey(e: KeyEventLike, st: KeyState): KeyResult {
  if (e.ctrlKey || e.metaKey) {
    st.pendingZ = false
    // ctrl 走全表;cmd 仅白名单键接管,其余 none(不得漏进单键映射——
    // cmd 按下时用户意图不是 vim 单键;cmd+w 已由原生菜单接管)
    const ok = e.ctrlKey || META_OK_KEYS.has(e.key)
    if (ok && /^[1-9]$/.test(e.key)) return { action: 'tabSwitch', arg: e.key }
    return (ok ? CTRL_MAP[e.key] : undefined) ?? { action: 'none' }
  }
  if (st.pendingZ) {
    st.pendingZ = false
    return Z_MAP[e.key] ?? SIMPLE_MAP[e.key] ?? { action: 'none' }
  }
  if (e.key === 'z') {
    st.pendingZ = true
    return { action: 'pending' }
  }
  return SIMPLE_MAP[e.key] ?? { action: 'none' }
}
