// 分栏渲染纯函数(验收反馈②):TUI logview.go 固定列宽模型的 TS 版。
// 列宽定宽 + 右侧补空格对齐;time 超宽取尾、logger 包名缩写、超宽截断加 …;
// level 只补不截(WARNING 不截);message 弹性不定宽。
// 宽度一律按显示宽计算(CJK 计 2),displayWidth 供换行引擎(Task 10)复用。
import type { LineDTO } from './types'

/** DEFAULT_WIDTHS 默认列宽(与 TUI columnWidths/padLevel 对齐:message 无宽)。 */
export const DEFAULT_WIDTHS: Record<string, number> = {
  time: 12,
  level: 5,
  thread: 10,
  traceId: 12,
  logger: 20,
}

/** HEADER_LABELS 表头中文标签。 */
const HEADER_LABELS: Record<string, string> = {
  time: '时间',
  level: '级别',
  thread: '线程',
  traceId: '追踪ID',
  logger: '日志器',
  message: '内容',
}

/** CJK_RANGES 显示宽 2 的区段(常用覆盖:日文假名/注音/CJK 统一/全角/半宽片假名等)。 */
const CJK_RANGES: [number, number][] = [
  [0x1100, 0x115f],
  [0x2e80, 0x303e],
  [0x3041, 0x33ff],
  [0x3400, 0x4dbf],
  [0x4e00, 0x9fff],
  [0xa000, 0xa4cf],
  [0xac00, 0xd7a3],
  [0xf900, 0xfaff],
  [0xfe30, 0xfe4f],
  [0xff00, 0xff60],
  [0xffe0, 0xffe6],
]

/** charW 单码点显示宽(CJK 区段计 2,其余 1)。 */
function charW(cp: number): number {
  for (const [lo, hi] of CJK_RANGES) {
    if (cp >= lo && cp <= hi) return 2
  }
  return 1
}

/** displayWidth 字符串显示宽(码点逐个累加,Task 10 换行引擎复用)。 */
export function displayWidth(s: string): number {
  let w = 0
  for (const ch of s) w += charW(ch.codePointAt(0) ?? 0)
  return w
}

/** abbreviateLogger 包名段一律取首字母,末段类名全保留(TUI logview.go:697)。 */
export function abbreviateLogger(s: string): string {
  const parts = s.split('.')
  if (parts.length <= 1) return s
  const head = parts
    .slice(0, -1)
    .filter((p) => p !== '')
    .map((p) => p[0] + '.')
    .join('')
  return head + parts[parts.length - 1]
}

/** tailVisual 取末 maxW 显示宽的子串。 */
function tailVisual(s: string, maxW: number): string {
  const cps = [...s]
  let w = 0
  let start = cps.length
  for (let i = cps.length - 1; i >= 0; i--) {
    const cw = charW(cps[i].codePointAt(0) ?? 0)
    if (w + cw > maxW) break
    w += cw
    start = i
  }
  return cps.slice(start).join('')
}

/** headVisual 取前 maxW 显示宽的子串(超过即停,不切半宽字符)。 */
function headVisual(s: string, maxW: number): string {
  const cps = [...s]
  let w = 0
  let end = 0
  for (const cp of cps) {
    const cw = charW(cp.codePointAt(0) ?? 0)
    if (w + cw > maxW) break
    w += cw
    end++
  }
  return cps.slice(0, end).join('')
}

/**
 * fitField 单字段定宽适配:
 * - level 只补不截(WARNING 不截断,宽可超)
 * - 空值 = width 个空格占位(列对齐不塌)
 * - time 超宽取尾(保时分秒);logger 先缩写
 * - 超宽截断加 …(按显示宽),右侧补空格到 width
 */
export function fitField(field: string, val: string, width: number): string {
  if (field === 'level') {
    return displayWidth(val) >= width ? val : val + ' '.repeat(width - displayWidth(val))
  }
  if (!val) return ' '.repeat(width)
  let v = field === 'logger' ? abbreviateLogger(val) : val
  if (field === 'time' && displayWidth(v) > width) {
    v = tailVisual(v, width)
  } else if (displayWidth(v) > width) {
    v = headVisual(v, width - 1) + '…'
  }
  return v + ' '.repeat(width - displayWidth(v))
}

export interface Cell {
  field: string
  text: string
}

/** layoutLine 一行分栏:fields 为有序可见字段;message 原文不定宽,其余走 fitField。 */
export function layoutLine(ln: LineDTO, fields: string[], widths: Record<string, number>): Cell[] {
  return fields.map((f) => ({
    field: f,
    text:
      f === 'message'
        ? ln.message
        : fitField(f, (ln as unknown as Record<string, string>)[f] ?? '', widths[f] ?? DEFAULT_WIDTHS[f] ?? 0),
  }))
}

/** headerCells 表头行:中文标签与数据列同宽对齐(message 列同样不定宽)。 */
export function headerCells(fields: string[], widths: Record<string, number>): Cell[] {
  return fields.map((f) => ({
    field: f,
    text:
      f === 'message'
        ? HEADER_LABELS.message
        : fitField(f, HEADER_LABELS[f] ?? f, widths[f] ?? DEFAULT_WIDTHS[f] ?? 0),
  }))
}

/** fieldsWidths 合成列宽:默认起步,rules.yaml fields[].width>0 覆盖(与 TUI SetColumnWidths 同源语义)。 */
export function fieldsWidths(
  allFields: { name: string; visible: boolean; width?: number }[],
  _visible: string[]
): Record<string, number> {
  const w = { ...DEFAULT_WIDTHS }
  for (const f of allFields) {
    if (f.width && f.width > 0) w[f.name] = f.width
  }
  return w
}
