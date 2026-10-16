// 剩余周期（Countdown 设计的核心数据）：各页面共用，保证状态判定与颜色一致。
// 状态：ok（充裕）/ soon（7 天内到期）/ late（已过期）/ none（一次性买断或无到期日）

const DAY = 86400000
const CYCLE_DAYS = { day: 1, week: 7, month: 30.4375, year: 365.25 }
export const SOON_DAYS = 7

function startOfToday() {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d
}

export function daysLeft(s) {
  if (!s || !s.next_renewal_date) return null
  const due = new Date(s.next_renewal_date + 'T00:00:00')
  return Math.round((due - startOfToday()) / DAY)
}

export function statusOf(s) {
  if (!s || s.billing_type === 'one_time' || !s.next_renewal_date) return 'none'
  const d = daysLeft(s)
  if (d < 0) return 'late'
  if (d <= SOON_DAYS) return 'soon'
  return 'ok'
}

// 已走过的周期比例 0..1（过期为 1）；用于周期条填充
export function elapsed(s) {
  const st = statusOf(s)
  if (st === 'none') return 1
  if (st === 'late') return 1
  const total = (CYCLE_DAYS[s.cycle] || 30.4375) * Math.max(1, s.cycle_count || 1)
  const left = daysLeft(s)
  return Math.min(1, Math.max(0.02, 1 - left / total))
}

// 周期条元素需要的 class 与 style
export function lifeBar(s, i = 0) {
  return {
    cls: ['life', statusOf(s)],
    style: { '--d': Math.min(i, 12) * 45 + 'ms' },
    fill: { transform: `scaleX(${elapsed(s)})` }
  }
}

export function isImg(v) {
  return typeof v === 'string' && (v.startsWith('/') || v.startsWith('http') || v.startsWith('data:'))
}
