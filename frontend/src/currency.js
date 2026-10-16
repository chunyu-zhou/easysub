// 货币显示名：中文界面 `CODE 符号 中文名`，其它语言 `CODE 符号 English name`（缺哪个用另一个）
export function curName(c, locale) {
  if (!c) return ''
  return (String(locale || '').startsWith('zh') ? (c.name_zh || c.name) : (c.name || c.name_zh)) || ''
}
export function curLabel(c, locale) {
  if (!c) return ''
  return [c.code, c.symbol, curName(c, locale)].filter(Boolean).join(' ')
}
