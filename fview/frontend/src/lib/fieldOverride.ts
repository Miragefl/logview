// 单日志字段覆盖层(2026-11-03 spec 需求②):localStorage 按源全称(tab.info.info,
// 本地路径/user@host:path/ctx·ns·资源)键控,同源多 tab 共享同一覆盖;
// 默认层在 rules.yaml(fieldsStore),两层互不影响。损坏数据一律吞异常回落空。

const KEY = 'fview-fields-override'

/** loadOverrides 读全部覆盖(键=源全称,值=可见字段名数组)。 */
export function loadOverrides(): Record<string, string[]> {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return {}
    const v = JSON.parse(raw) as unknown
    if (typeof v !== 'object' || v === null || Array.isArray(v)) return {}
    // 值非数组的键过滤掉:坏值(外部损坏/未来 schema 残留)混进 configuredFields
    // 会让渲染链 TypeError 白屏且每次切源复现,无法自救(终审修复)
    const entries = Object.entries(v as Record<string, unknown>).filter(([, x]) => Array.isArray(x))
    return Object.fromEntries(entries) as Record<string, string[]>
  } catch {
    return {}
  }
}

/** persistOverrides 整体落盘(写失败保内存态,下次变更重试)。 */
export function persistOverrides(m: Record<string, string[]>): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(m))
  } catch {
    /* 隐私模式等写失败:保内存态 */
  }
}
