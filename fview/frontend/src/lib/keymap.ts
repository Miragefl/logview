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
  | 'bookmark'
  | 'gotoBookmarks'
  | 'fields'
  | 'lineNumbers'
  | 'help'
  | 'clear'
  | 'pending'
  | 'none'

export interface KeyEventLike {
  key: string
  ctrlKey: boolean
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
}

const CTRL_MAP: Record<string, KeyResult> = {
  d: { action: 'halfDown' },
  u: { action: 'halfUp' },
  f: { action: 'pageDown' },
  b: { action: 'pageUp' },
}

const Z_MAP: Record<string, KeyResult> = {
  t: { action: 'anchorTop' },
  z: { action: 'anchorMid' },
  b: { action: 'anchorBottom' },
}

/** resolveKey 按键 → 动作;st.pendingZ 由本函数维护(z 序列)。 */
export function resolveKey(e: KeyEventLike, st: KeyState): KeyResult {
  if (e.ctrlKey) {
    st.pendingZ = false
    return CTRL_MAP[e.key] ?? { action: 'none' }
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
