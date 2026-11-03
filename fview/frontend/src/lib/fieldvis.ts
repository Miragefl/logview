// 列按解析器能力显隐(2026-11-03 spec 方案 C):正则没有该命名组则列永不出现,
// 不随缓冲区数据抖动;有组只是行值空(稀疏列)照常显示。
// parserFields 来自 tab:meta 事件(后端 FieldProducer 断言),null=后端未发(老版本
// 或解析器未声明能力)——语义为"不裁剪",与空数组(纯文本只剩 message)严格区分。

/**
 * effectiveFields 合成实际显示列:
 * - parserFields 为 null → configured 原样返回(兼容)
 * - 否则 configured ∩ parserFields,message 恒在(没内容列的日志查看器没有意义)
 * - 保 configured 序(列顺序始终跟配置走)
 */
export function effectiveFields(configured: string[], parserFields: string[] | null): string[] {
  // message 恒显兜底:configured 缺 message(共享 rules.yaml 被 TUI/手改关掉)也尾部补上,
  // 否则内容列消失且设置页/F 均勾死无法自救(spec 拍板项②,终审修复)
  const base = configured.includes('message') ? configured : [...configured, 'message']
  if (parserFields === null) return base
  const cap = new Set(parserFields)
  cap.add('message')
  return base.filter((f) => cap.has(f))
}
