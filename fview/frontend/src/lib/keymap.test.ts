// vim 键位机测试(验收反馈④):以 TUI internal/tui/app.go handleNormalKeys 为唯一准绳。
// 级别档位是大写 E/W/I/D/A(TUI 同款);小写 e/w/d 释放给折叠/换行/详情。
import { describe, expect, it } from 'vitest'
import { resolveKey } from './keymap'

interface FakeEvent {
  key: string
  ctrlKey?: boolean
  metaKey?: boolean
  shiftKey?: boolean
}

function k(key: string, opts: Partial<FakeEvent> = {}) {
  return { key, ctrlKey: false, shiftKey: false, ...opts } as KeyboardEvent
}

describe('resolveKey 全表(TUI 对齐)', () => {
  const cases: [FakeEvent, string, string?][] = [
    // 导航
    [{ key: 'j' }, 'cursorDown'],
    [{ key: 'k' }, 'cursorUp'],
    [{ key: 'g' }, 'gotoTop'],
    [{ key: 'G' }, 'gotoBottom'],
    [{ key: 'H' }, 'viewportTop'],
    [{ key: 'M' }, 'viewportMid'],
    [{ key: 'L' }, 'viewportBottom'],
    [{ key: 'd', ctrlKey: true }, 'halfDown'],
    [{ key: 'u', ctrlKey: true }, 'halfUp'],
    [{ key: 'f', ctrlKey: true }, 'pageDown'],
    [{ key: 'b', ctrlKey: true }, 'pageUp'],
    [{ key: 'PageDown' }, 'pageDown'],
    [{ key: 'PageUp' }, 'pageUp'],
    [{ key: 'n' }, 'nextHit'],
    [{ key: 'N' }, 'prevHit'],
    // 弹窗三分区(TUI /、f、ctrl+h、ctrl+x)——h 让光标左移、x 让 WordsPopup 删词(vim-copy r2)
    [{ key: '/' }, 'search'],
    [{ key: 'f' }, 'search'],
    [{ key: 'h', ctrlKey: true }, 'highlight'],
    [{ key: 'x', ctrlKey: true }, 'hide'],
    [{ key: 'x' }, 'none'], // 裸 x 退役(WordsPopup 弹窗内删词保留)
    // 级别档位:大写
    [{ key: 'E' }, 'level', 'E'],
    [{ key: 'W' }, 'level', 'W'],
    [{ key: 'I' }, 'level', 'I'],
    [{ key: 'D' }, 'level', 'D'],
    [{ key: 'A' }, 'level', 'A'],
    // 功能键:折叠/换行腾挪到 ctrl+e/w(vim-copy),小写让给字符移动
    [{ key: 'e', ctrlKey: true }, 'fold'],
    [{ key: 'd' }, 'detail'],
    [{ key: 'v' }, 'visual'],
    [{ key: 'V' }, 'visualLine'],
    [{ key: 'y' }, 'yank'],
    [{ key: 'm' }, 'bookmark'],
    [{ key: "'" }, 'gotoBookmarks'],
    [{ key: 'F' }, 'fields'],
    [{ key: '#' }, 'lineNumbers'],
    [{ key: '?' }, 'help'],
    [{ key: 'C' }, 'clear'],
    [{ key: 'Escape' }, 'escape'],
    // 字符光标移动(vim-copy:h/l 一字符,w/b/e 词,0/^/$ 消息域)
    [{ key: 'h' }, 'cursorLeft'],
    [{ key: 'l' }, 'cursorRight'],
    [{ key: 'w' }, 'wordFwd'],
    [{ key: 'b' }, 'wordBack'],
    [{ key: 'e' }, 'wordEnd'],
    [{ key: '0' }, 'lineStart'],
    [{ key: '^' }, 'firstNonBlank'],
    [{ key: '$' }, 'lineEnd'],
    // 全键盘操作(第四轮验收③;ctrl+w 已改换行,关 tab 走 mac ⌘W 原生菜单)
    [{ key: 'q' }, 'sources'],
    [{ key: ',' }, 'settings'],
    [{ key: 'w', ctrlKey: true }, 'wrap'],
    [{ key: 'j', ctrlKey: true }, 'navDown'],
    [{ key: 'k', ctrlKey: true }, 'navUp'],
    [{ key: '=', ctrlKey: true }, 'zoomIn'],
    [{ key: '-', ctrlKey: true }, 'zoomOut'],
    [{ key: 'Enter' }, 'activate'],
    [{ key: 'a' }, 'none'], // 小写 a 不再是级别
    [{ key: 'i' }, 'none'],
    [{ key: 'z' }, 'pending'], // z 置 pending 态,见下组
  ]
  it.each(cases)('%j → %s %s', (ev, action, arg) => {
    const r = resolveKey(k(ev.key, ev), { pendingZ: false })
    expect(r.action).toBe(action)
    if (arg) expect(r.arg).toBe(arg)
  })

  it('z 序列状态机:z → pending;z 后 t/z/b → anchor*;z 后他键 → 正常解析且复位', () => {
    const st = { pendingZ: false }
    expect(resolveKey(k('z'), st).action).toBe('pending')
    expect(st.pendingZ).toBe(true)
    expect(resolveKey(k('t'), st)).toEqual({ action: 'anchorTop', arg: undefined })
    // 再次 z+zb
    resolveKey(k('z'), st)
    expect(resolveKey(k('b'), st).action).toBe('anchorBottom')
    resolveKey(k('z'), st)
    expect(resolveKey(k('z'), st).action).toBe('anchorMid') // zz 居中
    expect(st.pendingZ).toBe(false)
    // z 后按 j:j 正常生效,pendingZ 复位
    resolveKey(k('z'), st)
    expect(resolveKey(k('j'), st).action).toBe('cursorDown')
    expect(st.pendingZ).toBe(false)
    resolveKey(k('z'), st)
    expect(resolveKey(k('x', { ctrlKey: true }), st).action).toBe('hide') // r2:裸 x 退役
    expect(st.pendingZ).toBe(false)
  })

  it('ctrl 键不受 pendingZ 干扰;z+ctrl+d → halfDown', () => {
    const st = { pendingZ: false }
    resolveKey(k('z'), st)
    expect(resolveKey(k('d', { ctrlKey: true }), st).action).toBe('halfDown')
    expect(st.pendingZ).toBe(false)
  })

  it('ctrl+1..9 切 tab,arg 带序号;ctrl+0 无绑定(第四轮验收③)', () => {
    const st = { pendingZ: false }
    expect(resolveKey(k('1', { ctrlKey: true }), st)).toEqual({ action: 'tabSwitch', arg: '1' })
    expect(resolveKey(k('9', { ctrlKey: true }), st)).toEqual({ action: 'tabSwitch', arg: '9' })
    expect(resolveKey(k('0', { ctrlKey: true }), st).action).toBe('none')
  })

  it('cmd 系键位(第七轮验收①③):cmd+=/- 字号、cmd+, 设置;cmd+w/j/k 不接(系统键不可拦)', () => {
    const st = { pendingZ: false }
    expect(resolveKey(k('=', { metaKey: true }), st).action).toBe('zoomIn')
    expect(resolveKey(k('+', { metaKey: true }), st).action).toBe('zoomIn')
    expect(resolveKey(k('-', { metaKey: true }), st).action).toBe('zoomOut')
    expect(resolveKey(k(',', { metaKey: true }), st).action).toBe('settings')
    expect(resolveKey(k(',', { ctrlKey: true }), st).action).toBe('settings')
    expect(resolveKey(k('1', { metaKey: true }), st)).toEqual({ action: 'tabSwitch', arg: '1' })
    // cmd 系统键不接管:cmd+w 关窗/cmd+j 等交给系统,应用不拦
    expect(resolveKey(k('w', { metaKey: true }), st).action).toBe('none')
    expect(resolveKey(k('j', { metaKey: true }), st).action).toBe('none')
    expect(resolveKey(k('d', { metaKey: true }), st).action).toBe('none')
  })
})

// ⌘C/Ctrl+C 复制选区(第九轮验收④):mac ⌘C 必须接管(META 白名单加 c);
// 有无选区的拦截/放行由 App 层决定,keymap 只管解析。
describe('copySelection 解析', () => {
  it('meta+c 与 ctrl+c 均解析为 copySelection', () => {
    expect(resolveKey(k('c', { metaKey: true }), { pendingZ: false })).toEqual({ action: 'copySelection' })
    expect(resolveKey(k('c', { ctrlKey: true }), { pendingZ: false })).toEqual({ action: 'copySelection' })
  })
})

// y 序列状态机(vim-copy):单 y 不再立即复制——y 进 pendingY 态;
// yy=复制整行(yankLine),y 后其他键透传正常解析(pendingY 复位)。
describe('pendingY 状态机', () => {
  it('单按 y → yank 且置 pendingY;visual 态由 App 层消化', () => {
    const st = { pendingZ: false, pendingY: false }
    expect(resolveKey(k('y'), st)).toEqual({ action: 'yank' })
    expect(st.pendingY).toBe(true)
  })

  it('yy → yankLine 且 pendingY 复位;第三次 y 重新进 pending', () => {
    const st = { pendingZ: false, pendingY: false }
    resolveKey(k('y'), st)
    expect(resolveKey(k('y'), st)).toEqual({ action: 'yankLine' })
    expect(st.pendingY).toBe(false)
    // yankLine 后再按 y:新一轮 pending
    expect(resolveKey(k('y'), st).action).toBe('yank')
    expect(st.pendingY).toBe(true)
  })

  it('y 后其他键透传正常解析且 pendingY 复位', () => {
    const st = { pendingZ: false, pendingY: false }
    resolveKey(k('y'), st)
    expect(resolveKey(k('j'), st).action).toBe('cursorDown')
    expect(st.pendingY).toBe(false)
    resolveKey(k('y'), st)
    expect(resolveKey(k('$'), st).action).toBe('lineEnd')
    expect(st.pendingY).toBe(false)
  })

  it('y 后 ctrl 键透传:y+ctrl+d → halfDown', () => {
    const st = { pendingZ: false, pendingY: false }
    resolveKey(k('y'), st)
    expect(resolveKey(k('d', { ctrlKey: true }), st).action).toBe('halfDown')
    expect(st.pendingY).toBe(false)
  })

  it('y 后 Esc → escape 且 pendingY 复位', () => {
    const st = { pendingZ: false, pendingY: false }
    resolveKey(k('y'), st)
    expect(resolveKey(k('Escape'), st).action).toBe('escape')
    expect(st.pendingY).toBe(false)
  })

  it('y 后 z 进 z 序列:y+zt → anchorTop', () => {
    const st = { pendingZ: false, pendingY: false }
    resolveKey(k('y'), st)
    expect(resolveKey(k('z'), st).action).toBe('pending')
    expect(st.pendingZ).toBe(true)
    expect(st.pendingY).toBe(false)
    expect(resolveKey(k('t'), st).action).toBe('anchorTop')
  })

  it('z 序列内 y 走表解析,不触发 pendingY(嵌套序列禁止)', () => {
    const st = { pendingZ: false, pendingY: false }
    resolveKey(k('z'), st)
    expect(resolveKey(k('y'), st).action).toBe('yank')
    expect(st.pendingY).toBe(false) // z 序列透传不设 pendingY
    expect(st.pendingZ).toBe(false)
  })
})
