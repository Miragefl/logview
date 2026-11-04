// highlight 子序列匹配与高亮切分(r2 打字过滤→r3 非连续):fzf 式——
// "hardware" 输 "hde" 命中(h…d…e 按序出现),连续命中是子集(视觉不变)。
// segments 插值渲染(不用 v-html,资源名来自 kubectl/文件系统防注入)。
export interface HlSeg {
  t: string
  hit: boolean
}

/** matchPositions 子串优先匹配(大小写不敏感):完整子串命中→整段连续下标(r2 视觉
 * 不变——贪心会把 'my app' 的 m 吃到 '/tmp' 的 m 上,子串优先才保整段);
 * 无子串退化为子序列贪心(fzf 式非连续,hde→hardware)。词空返回 []、不命中/词长于
 * 文本返回 null——isSubseq 与 splitHighlight 共用同一位置算法(过滤与高亮见到的
 * 命中位置必须一致)。 */
function matchPositions(text: string, kw: string): number[] | null {
  const k = kw.toLowerCase()
  if (!k.trim()) return []
  const lower = text.toLowerCase()
  const at = lower.indexOf(k)
  if (at >= 0) return Array.from({ length: k.length }, (_, i) => at + i)
  const pos: number[] = []
  let i = 0
  for (let j = 0; j < lower.length; j++) {
    if (i < k.length && lower[j] === k[i]) {
      pos.push(j)
      i++
    }
  }
  return i === k.length ? pos : null
}

/** isSubseq 子序列判定(面板 items 过滤谓词):kw 各字符按序出现于 text 即命中;
 * 空词恒真(不过滤);不 trim——空格是路径/主机名合法字符。 */
export function isSubseq(text: string, kw: string): boolean {
  return matchPositions(text, kw) !== null
}

/** splitHighlight 子序列高亮切分(保留原文大小写):命中字符标 hit,相邻命中合并成段
 * (连续命中=整段,r2 视觉不变;非连续各字符单独成段);词空 identity;不命中整体 miss。 */
export function splitHighlight(text: string, kw: string): HlSeg[] {
  const pos = matchPositions(text, kw)
  if (pos === null) return [{ t: text, hit: false }]
  const hits = new Array<boolean>(text.length).fill(false)
  for (const j of pos) hits[j] = true
  // 相邻同态合并
  const segs: HlSeg[] = []
  let start = 0
  for (let j = 1; j <= text.length; j++) {
    if (j === text.length || hits[j] !== hits[start]) {
      segs.push({ t: text.slice(start, j), hit: hits[start] })
      start = j
    }
  }
  return segs
}
