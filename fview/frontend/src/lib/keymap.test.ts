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
    // 弹窗三分区(TUI /、f、h、x)
    [{ key: '/' }, 'search'],
    [{ key: 'f' }, 'search'],
    [{ key: 'h' }, 'highlight'],
    [{ key: 'x' }, 'hide'],
    // 级别档位:大写
    [{ key: 'E' }, 'level', 'E'],
    [{ key: 'W' }, 'level', 'W'],
    [{ key: 'I' }, 'level', 'I'],
    [{ key: 'D' }, 'level', 'D'],
    [{ key: 'A' }, 'level', 'A'],
    // 功能键(小写释放)
    [{ key: 'e' }, 'fold'],
    [{ key: 'd' }, 'detail'],
    [{ key: 'w' }, 'wrap'],
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
    // 全键盘操作(第四轮验收③)
    [{ key: 'q' }, 'sources'],
    [{ key: ',' }, 'settings'],
    [{ key: 'w', ctrlKey: true }, 'closeTab'],
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
    expect(resolveKey(k('x'), st).action).toBe('hide')
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
