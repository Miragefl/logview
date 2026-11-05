// vim 键位机(验收反馈④):纯函数按键 → 动作,以 TUI internal/tui/app.go
// handleNormalKeys 为唯一准绳。级别档位 = 大写 E/W/I/D/A。vim-copy(2026-11-05):
// h/l/w/b/e/0/$/^ 让给字符光标移动(message 域),折叠/换行/高亮腾挪到
// ctrl+e/w/h。z 序列(zt/zz/zb)走 pendingZ、y 序列(yy)走 pendingY 状态机,
// 状态由调用方持有。
export type KeyAction =
  | 'cursorDown'
  | 'cursorUp'
  | 'cursorLeft'
  | 'cursorRight'
  | 'wordFwd'
  | 'wordBack'
  | 'wordEnd'
  | 'lineStart'
  | 'firstNonBlank'
  | 'lineEnd'
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
  | 'yankLine'
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
  /** y 序列态(yy=复制整行);可选字段——存量调用点仅初始化 pendingZ,内部 !! 归一 */
  pendingY?: boolean
}

export interface KeyResult {
  action: KeyAction
  /** level 档位 arg ∈ E|W|I|D|A */
  arg?: string
}

/** SIMPLE_MAP 无状态单键映射(z/y 序列与 ctrl 组合另查;y 在此仅查表返回,
 *  pendingY 置位只在主路径 resolveInner 做,z 序列透传不嵌套触发)。 */
const SIMPLE_MAP: Record<string, KeyResult> = {
  j: { action: 'cursorDown' },
  k: { action: 'cursorUp' },
  h: { action: 'cursorLeft' },
  l: { action: 'cursorRight' },
  w: { action: 'wordFwd' },
  b: { action: 'wordBack' },
  e: { action: 'wordEnd' },
  '0': { action: 'lineStart' },
  '^': { action: 'firstNonBlank' },
  $: { action: 'lineEnd' },
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
  E: { action: 'level', arg: 'E' },
  W: { action: 'level', arg: 'W' },
  I: { action: 'level', arg: 'I' },
  D: { action: 'level', arg: 'D' },
  A: { action: 'level', arg: 'A' },
  d: { action: 'detail' },
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
  w: { action: 'wrap' },
  h: { action: 'highlight' },
  e: { action: 'fold' },
  x: { action: 'hide' }, // r2:裸 x 退役,隐藏弹窗挪 ctrl+x(裸 x 留给 WordsPopup 删词域)
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

/** resolveKey 按键 → 动作;st.pendingZ/pendingY 由本函数维护(z/y 序列)。
 *  ctrl+1..9 切 tab(浏览器惯例,第四轮验收③全键盘);
 *  cmd 系白名单键(字号/设置/tab 数字)与 ctrl 等价,其余 cmd 交给系统。
 *  pendingY 透传:y 后其他键按无 y 前缀重新解析(vim 手感,不弹提示)。 */
export function resolveKey(e: KeyEventLike, st: KeyState): KeyResult {
  if (e.ctrlKey || e.metaKey) {
    st.pendingZ = false
    st.pendingY = false
    // ctrl 走全表;cmd 仅白名单键接管,其余 none(不得漏进单键映射——
    // cmd 按下时用户意图不是 vim 单键;cmd+w 已由原生菜单接管)
    const ok = e.ctrlKey || META_OK_KEYS.has(e.key)
    if (ok && /^[1-9]$/.test(e.key)) return { action: 'tabSwitch', arg: e.key }
    return (ok ? CTRL_MAP[e.key] : undefined) ?? { action: 'none' }
  }
  if (st.pendingY) {
    st.pendingY = false
    if (e.key === 'y') return { action: 'yankLine' }
    // 透传:无 y 前缀重新解析(含进 z 序列)
    return resolveInner(e, st)
  }
  if (st.pendingZ) {
    st.pendingZ = false
    // z 序列查表:y 走 SIMPLE_MAP 返回 yank 但不置 pendingY(序列不嵌套)
    return Z_MAP[e.key] ?? SIMPLE_MAP[e.key] ?? { action: 'none' }
  }
  return resolveInner(e, st)
}

/** resolveInner 非序列态单键解析:z/y 进各自序列,其余查表。 */
function resolveInner(e: KeyEventLike, st: KeyState): KeyResult {
  if (e.key === 'z') {
    st.pendingZ = true
    return { action: 'pending' }
  }
  if (e.key === 'y') {
    st.pendingY = true
    return { action: 'yank' }
  }
  return SIMPLE_MAP[e.key] ?? { action: 'none' }
}
