// ANSI SGR 转义 → HTML span（虚拟滚动行渲染用；只映射日志常见 SGR：
// 30-37/90-97 前景色、1 加粗、0 重置，其余码忽略）

const FG_COLORS = [
  'black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white',
] as const

const SGR_RE = /\x1b\[([0-9;]*)m/g

function classNames(codes: string[]): string {
  const cls: string[] = []
  for (const c of codes) {
    const n = Number(c)
    if (n === 0) continue
    if (n === 1) cls.push('bold')
    if (n >= 30 && n <= 37) cls.push(`fg-${FG_COLORS[n - 30]}`)
    if (n >= 90 && n <= 97) cls.push(`fg-bright-${FG_COLORS[n - 90]}`)
  }
  return cls.join(' ')
}

/** ansiToHtml 把 ANSI 着色文本转为带 class 的 span 序列；无转义文本原样返回。 */
export function ansiToHtml(s: string): string {
  if (!s.includes('\x1b[')) return s
  let out = ''
  let last = 0
  let current = ''
  SGR_RE.lastIndex = 0
  let m: RegExpExecArray | null
  while ((m = SGR_RE.exec(s)) !== null) {
    const text = s.slice(last, m.index)
    if (text) {
      out += current ? `<span class="${current}">${text}</span>` : text
    }
    last = m.index + m[0].length
    // 空码（"\x1b[m"）视为 reset
    const body = m[1] === '' ? '0' : m[1]
    for (const c of body.split(';')) {
      if (c === '0' || c === '') {
        current = ''
      } else {
        // 逐码累积样式（组合码 \x1b[1;31m 与顺序等效）
        const one = classNames([c])
        current = mergeClass(current, one)
      }
    }
  }
  const tail = s.slice(last)
  if (tail) {
    out += current ? `<span class="${current}">${tail}</span>` : tail
  }
  return out
}

function mergeClass(cur: string, add: string): string {
  if (!add) return cur
  if (!cur) return add
  return `${cur} ${add}`
}
