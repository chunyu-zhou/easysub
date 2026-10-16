<template>
  <div class="notify-page">
    <div class="page-head">
      <h1>{{ t('notify.title') }}</h1>
      <button class="btn" @click="runScan"><span v-html="icon('send')"></span>{{ t('notify.runScan') }}</button>
    </div>

    <div class="card list-card">
      <div v-if="logs.length" class="n-list">
        <div v-for="l in logs" :key="l.id" class="n-row">
          <div class="n-top">
            <span class="tag" :class="l.status === 'sent' ? 'ok' : 'late'">
              {{ l.status === 'sent' ? t('notify.sent') : t('notify.failed') }}
            </span>
            <span v-if="l.channel" class="tag">{{ l.channel }}</span>
            <span class="days">{{ t('notify.daysBefore') }}: <b>{{ l.days_before }}</b></span>
            <span class="time">{{ fmt(l.sent_at) }}</span>
          </div>
          <div class="n-body">{{ l.message }}</div>
        </div>
      </div>
      <div v-else class="empty">
        <span class="empty-i" v-html="icon('inbox')"></span>
        <b>{{ t('notify.empty') }}</b>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { icon } from '../icons'
import api from '../api'

const { t } = useI18n()
const logs = ref([])

function fmt(s) { return s ? new Date(s).toLocaleString() : '' }

async function load() { logs.value = (await api.get('/api/notifications/logs')).data }
async function runScan() { await api.post('/api/notifications/run-scan'); load() }

onMounted(load)
</script>

<style scoped>
.notify-page { min-width: 0; }
.btn span { display: inline-flex; }
.list-card { padding: 4px 0; }
.n-row { padding: 14px 20px; border-bottom: 1px solid var(--border); min-width: 0; }
.n-row:last-child { border-bottom: 0; }
.n-top { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 10px; margin-bottom: 6px; }
.days { font-size: 12.5px; color: var(--text-soft); }
.days b { color: var(--text); font-weight: 600; font-variant-numeric: tabular-nums; }
.time { margin-left: auto; font-family: var(--mono); font-size: 12px; color: var(--text-soft);
  font-variant-numeric: tabular-nums; white-space: nowrap; }
.n-body { font-size: 14px; line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere; color: var(--text); }
.empty-i { display: inline-flex; color: var(--text-faint); }

@media (max-width: 720px) {
  .list-card { padding: 0; }
  .n-row { padding: 12px 14px; }
  .time { margin-left: 0; flex-basis: 100%; }
  .page-head .btn { width: 100%; }
}
</style>
