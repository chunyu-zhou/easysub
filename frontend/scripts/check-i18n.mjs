/**
 * i18n 词条体检：在 build 之前跑（package.json 的 prebuild 钩子），拦住只在浏览器里才炸的问题。
 *
 * 为什么需要它：vue-i18n 的消息是运行时编译的，语法错误在 vite build 阶段完全看不出来，
 * 但页面一渲染就抛异常、整块组件被卸载 —— 表现就是「点某个标签页后整栏消失」（debug13 / debug16）。
 *
 * 检查项：
 *   1. 编译失败（致命）：常见于文案里出现裸 @（链接消息标记，如 @all）。用 {'@'} 转义。
 *   2. 含竖线（警告）：| 是复数分隔符，t() 会只返回第一段，后半句被静默截断。用 {'|'} 转义。
 *   3. 各语言键缺失（警告）：以中文为基准，列出 en / ru 缺的键。
 */
import { baseCompile } from '@intlify/message-compiler'

globalThis.localStorage = { getItem: () => 'zh', setItem: () => {} }
const i18n = (await import('../src/i18n/index.js')).default
const messages = i18n.global.messages.value ?? i18n.global.messages

/** 把嵌套的 messages 摊平成 { 'a.b.c': '文案' } */
function flatten(obj, prefix = '', out = {}) {
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object' && !Array.isArray(v)) flatten(v, key, out)
    else if (typeof v === 'string') out[key] = v
  }
  return out
}

const flat = Object.fromEntries(
  Object.keys(messages).map((loc) => [loc, flatten(messages[loc])])
)
const locales = Object.keys(flat)
const errors = []
const warnings = []

for (const loc of locales) {
  for (const [key, text] of Object.entries(flat[loc])) {
    try {
      baseCompile(text, { onError: (e) => { throw e } })
    } catch (e) {
      errors.push(`[${loc}] ${key} —— ${e.message}\n      ${text.slice(0, 100)}`)
    }
    // 先剔掉 {'...'} 字面量插值，避免把已经转义好的 {'|'} 误报成裸竖线
    if (text.replace(/\{'[^']*'\}/g, '').includes('|')) {
      warnings.push(`[${loc}] ${key} 含裸竖线，t() 会在此截断，请写成 {'|'}`)
    }
  }
}

// 键覆盖率：以第一个语言（zh）为基准
const [base, ...others] = locales
for (const loc of others) {
  const missing = Object.keys(flat[base]).filter((k) => !(k in flat[loc]))
  if (missing.length) {
    warnings.push(`[${loc}] 缺 ${missing.length} 个键：${missing.slice(0, 8).join(', ')}${missing.length > 8 ? ' ...' : ''}`)
  }
}

const total = locales.reduce((n, loc) => n + Object.keys(flat[loc]).length, 0)
console.log(`i18n 体检：${locales.length} 种语言、${total} 条词条`)
for (const w of warnings) console.log(`⚠️  ${w}`)
if (errors.length) {
  console.error(`\n❌ ${errors.length} 条词条编译失败（页面渲染时会抛异常并让整块组件消失）：`)
  for (const e of errors) console.error(`   ${e}`)
  console.error(`\n修法：文案里的 @ 写成 {'@'}、| 写成 {'|'}（vue-i18n 字面量插值）。`)
  process.exit(1)
}
console.log(`✅ 全部通过${warnings.length ? `（${warnings.length} 条警告）` : ''}`)
