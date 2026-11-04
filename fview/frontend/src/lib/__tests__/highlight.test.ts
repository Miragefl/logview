// splitHighlight 纯函数测试(r2 打字过滤→r3 子序列):大小写不敏感子序列切分/非连续命中/
// 相邻命中合并/多次命中/空词 identity/不命中;isSubseq 贪心匹配。
import { describe, expect, it } from 'vitest'
import { isSubseq, splitHighlight } from '../highlight'

describe('splitHighlight 高亮切分', () => {
  it('空词/全空格词 identity(整体 miss)', () => {
    expect(splitHighlight('/tmp/app.log', '')).toEqual([{ t: '/tmp/app.log', hit: false }])
    expect(splitHighlight('/tmp/app.log', '   ')).toEqual([{ t: '/tmp/app.log', hit: false }])
  })

  it('连续命中整段高亮(相邻命中字符合并——r2 视觉不变)', () => {
    expect(splitHighlight('/tmp/error.log', 'error')).toEqual([
      { t: '/tmp/', hit: false },
      { t: 'error', hit: true },
      { t: '.log', hit: false },
    ])
  })

  it('非连续子序列命中——hardware 输 hde 各命中字符高亮(fzf 式)', () => {
    // h-0 a-1 r-2 d-3 w-4 a-5 r-6 e-7:h…d…e 按序命中
    expect(splitHighlight('hardware', 'hde')).toEqual([
      { t: 'h', hit: true },
      { t: 'ar', hit: false },
      { t: 'd', hit: true },
      { t: 'war', hit: false },
      { t: 'e', hit: true },
    ])
  })

  it('相邻命中字符合并成段', () => {
    // aab 输 aa:前两个 a 相邻合并
    expect(splitHighlight('aab', 'aa')).toEqual([
      { t: 'aa', hit: true },
      { t: 'b', hit: false },
    ])
  })

  it('大小写不敏感(词大写命中文本小写,保留原文大小写)', () => {
    expect(splitHighlight('order-Svc', 'SVC')).toEqual([
      { t: 'order-', hit: false },
      { t: 'Svc', hit: true },
    ])
  })

  it('词字符各消费一次——同词第二处出现不再命中(子序列语义)', () => {
    expect(splitHighlight('app-error-app', 'app')).toEqual([
      { t: 'app', hit: true },
      { t: '-error-app', hit: false },
    ])
    // 多字符分散命中:abab 输 bb → 两段各命中
    expect(splitHighlight('abab', 'bb')).toEqual([
      { t: 'a', hit: false },
      { t: 'b', hit: true },
      { t: 'a', hit: false },
      { t: 'b', hit: true },
    ])
  })

  it('不命中整体 miss;命中在首/尾', () => {
    expect(splitHighlight('abc', 'zzz')).toEqual([{ t: 'abc', hit: false }])
    expect(splitHighlight('devbox', 'dev')).toEqual([
      { t: 'dev', hit: true },
      { t: 'box', hit: false },
    ])
    expect(splitHighlight('web1', '1')).toEqual([
      { t: 'web', hit: false },
      { t: '1', hit: true },
    ])
  })

  it('词含空格不 trim(路径合法字符,子序列同样适用)', () => {
    expect(splitHighlight('/tmp/my app.log', 'my app')).toEqual([
      { t: '/tmp/', hit: false },
      { t: 'my app', hit: true },
      { t: '.log', hit: false },
    ])
    // 非连续+空格:m(2) y(6) p(9) 贪心最早匹配(fzf 式)
    expect(splitHighlight('/tmp/my app.log', 'myp')).toEqual([
      { t: '/t', hit: false },
      { t: 'm', hit: true },
      { t: 'p/m', hit: false },
      { t: 'y', hit: true },
      { t: ' a', hit: false },
      { t: 'p', hit: true },
      { t: 'p.log', hit: false },
    ])
  })
})

describe('isSubseq 子序列判定(面板过滤谓词)', () => {
  it('连续/非连续/大小写', () => {
    expect(isSubseq('hardware', 'hde')).toBe(true)
    expect(isSubseq('hardware', 'hardware')).toBe(true)
    expect(isSubseq('Hardware', 'HDE')).toBe(true)
    expect(isSubseq('hardware', 'dhh')).toBe(false) // 顺序错
    expect(isSubseq('hardware', 'zzz')).toBe(false)
  })

  it('空词恒真(不过滤);词长于文本恒假', () => {
    expect(isSubseq('abc', '')).toBe(true)
    expect(isSubseq('abc', 'abcd')).toBe(false)
  })
})
