<template>
  <div class="logs-page">
    <div class="page-head">
      <h1>{{ t('rtlog.title') }}</h1>
      <label class="live" :class="{ on: live }">
        <input type="checkbox" v-model="live" class="sr" />
        <span class="dot" :class="{ ok: live }"></span>
        {{ live ? t('rtlog.live') : t('rtlog.paused') }}
      </label>
    </div>

    <div class="card stream-card">
      <div v-if="logs.length" class="log-stream">
        <div v-for="l in logs" :key="l.id" class="log-line">
          <div class="lh">
            <span class="time">{{ fmt(l.created_at) }}</span>
            <span class="tag lvl" :class="{ soon: l.level === 'warn', late: l.level === 'error' }">{{ l.level }}</span>
          </div>
          <div class="lw">
            <span class="act">{{ l.action }}</span>
            <span v-if="l.user" class="usr">@{{ l.user }}</span>
          </div>
          <div class="detail">{{ l.detail }}</div>
        </div>
      </div>
      <div v-else class="empty">
        <span class="empty-i" v-html="icon('logs')"></span>
        <b>{{ t('rtlog.empty') }}</b>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { icon } from '../icons'
import api from '../api'

const { t } = useI18n()
const logs = ref([])
const live = ref(true)
let lastId = 0
let timer = null

function fmt(s) { return s ? new Date(s).toLocaleTimeString() : '' }

async function initial() {
  const { data } = await api.get('/api/logs', { params: { limit: 100 } })
  logs.value = data
  lastId = data.length ? data[data.length - 1].id : 0
}

async function poll() {
  if (!live.value) return
  try {
    const { data } = await api.get('/api/logs', { params: { after: lastId } })
    if (data.length) {
      logs.value.push(...data)
      lastId = data[data.length - 1].id
      if (logs.value.length > 400) logs.value.splice(0, logs.value.length - 400)
    }
  } catch { /* ignore transient errors */ }
}

watch(live, (v) => { if (v) poll() })

onMounted(async () => {
  await initial()
  timer = setInterval(poll, 4000)
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.logs-page { min-width: 0; }

/* 实时开关 */
.live { position: relative; display: inline-flex; align-items: center; gap: 8px; margin: 0; min-height: 40px;
  padding: 0 14px; border: 1px solid var(--border); border-radius: 999px; background: var(--surface);
  font-size: 13px; font-weight: 550; color: var(--text-soft); cursor: pointer; user-select: none;
  transition: border-color .15s, color .15s; }
.live:hover { border-color: var(--border-strong); color: var(--text); }
.live.on { color: var(--text); }
.live:focus-within { box-shadow: var(--focus); }
.sr { position: absolute; width: 1px; height: 1px; opacity: 0; margin: 0; pointer-events: none; }
.live .dot.ok { animation: live-pulse 1.6s ease-in-out infinite; }
@keyframes live-pulse { 0%, 100% { opacity: 1; } 50% { opacity: .35; } }

/* 日志流 */
.stream-card { padding: 4px 0; }
.log-stream { max-height: 70vh; overflow-y: auto; overflow-x: hidden; overscroll-behavior: contain; }
.log-line { display: flex; align-items: baseline; gap: 6px 14px; padding: 8px 18px; font-size: 13px;
  border-bottom: 1px solid var(--border); min-width: 0; }
.log-line:last-child { border-bottom: 0; }
.log-line:hover { background: color-mix(in srgb, var(--surface-2) 60%, transparent); }
.lh { display: flex; align-items: baseline; gap: 10px; flex: 0 0 auto; }
.time { font-family: var(--mono); font-size: 12px; color: var(--text-soft); font-variant-numeric: tabular-nums;
  white-space: nowrap; min-width: 8.5ch; }
.lvl { font-size: 11px; padding: 0 8px; min-width: 48px; justify-content: center; }
.lw { display: flex; align-items: baseline; gap: 4px 8px; flex: 0 1 240px; min-width: 0; flex-wrap: wrap; }
.act { font-family: var(--mono); font-size: 12.5px; font-weight: 600; color: var(--text); overflow-wrap: anywhere; min-width: 0; }
.usr { font-size: 12.5px; color: var(--text-soft); overflow-wrap: anywhere; min-width: 0; }
.detail { flex: 1 1 0; min-width: 0; color: var(--text-soft); line-height: 1.5; white-space: pre-wrap;
  overflow-wrap: anywhere; word-break: break-word; }

.empty-i { display: inline-flex; color: var(--text-faint); }

/* 手机端：每条日志分三行堆叠 —— 时间+级别 / 操作+用户 / 详情，无固定宽度列 */
@media (max-width: 720px) {
  .stream-card { padding: 0; }
  .log-stream { max-height: none; overflow: visible; }
  .log-line { flex-direction: column; align-items: stretch; gap: 4px; padding: 11px 14px; }
  .lh { justify-content: space-between; }
  .time { min-width: 0; }
  .lw { flex: none; }
  .detail { flex: none; font-size: 13px; }
}
</style>
