<template>
  <div class="setup">
    <div class="card setup-card">
      <div class="wordmark"><span class="mark"><i></i></span><span>EasySub</span></div>
      <h1>{{ t('setup.title') }}</h1>
      <p class="sub">{{ t('setup.subtitle') }}</p>

      <section class="storage" aria-live="polite">
        <div class="storage-icon" v-html="icon('library')"></div>
        <div>
          <b>{{ t('setup.sqlite') }}</b>
          <p>{{ t('setup.sqliteDesc') }}</p>
        </div>
      </section>

      <label for="st-path">{{ t('setup.sqlitePath') }}</label>
      <input id="st-path" v-model="cfg.path" class="mono" placeholder="easysub.db" @input="resetTest" />
      <p class="tip"><span class="tip-i" v-html="icon('note')"></span><span>{{ t('setup.sqliteTip') }}</span></p>

      <p v-if="msg" class="note" :class="ok ? 'ok' : 'late'" role="status">{{ msg }}</p>

      <div class="acts">
        <button class="btn ghost" :disabled="busy" @click="testStorage">
          {{ testing ? t('setup.testing') : t('setup.test') }}
        </button>
        <button class="btn" :disabled="busy || !ok" @click="save">
          {{ saving ? t('setup.saving') : t('setup.save') }}
        </button>
      </div>

      <div class="lang">
        <button :class="{ on: locale === 'zh' }" @click="setLang('zh')">中文</button>
        <button :class="{ on: locale === 'en' }" @click="setLang('en')">EN</button>
        <button :class="{ on: locale === 'ru' }" @click="setLang('ru')">RU</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { icon } from '../icons'
import axios from 'axios'

const { t, locale } = useI18n()
const cfg = reactive({ path: 'easysub.db' })
const msg = ref('')
const ok = ref(false)
const testing = ref(false)
const saving = ref(false)
const busy = ref(false)

function resetTest() { ok.value = false; msg.value = '' }
function setLang(l) { locale.value = l; localStorage.setItem('locale', l) }

async function testStorage() {
  busy.value = true; testing.value = true; msg.value = ''; ok.value = false
  try {
    const { data } = await axios.post('/api/setup/test', cfg)
    ok.value = true; msg.value = `${t('setup.testOk')}: ${data.message || ''}`
  } catch (e) {
    msg.value = `${t('setup.testFail')}: ${e.response?.data?.detail || e.message}`
  } finally { busy.value = false; testing.value = false }
}

async function save() {
  busy.value = true; saving.value = true; msg.value = ''
  try {
    await axios.post('/api/setup/save', cfg)
    msg.value = t('setup.done')
    setTimeout(() => { window.location.href = '/login' }, 800)
  } catch (e) {
    ok.value = false; msg.value = `${e.response?.data?.detail || e.message}`
    busy.value = false; saving.value = false
  }
}
</script>

<style scoped>
.setup { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 32px 16px; background: var(--bg); }
.setup-card { width: 100%; max-width: 460px; padding: 28px; }
.wordmark { display: inline-flex; align-items: center; gap: 10px; font-weight: 700; font-size: 17px; letter-spacing: -.02em; margin-bottom: 22px; }
.mark { width: 30px; height: 30px; border-radius: 9px; background: var(--primary); display: inline-flex; align-items: center; padding: 0 6px; flex-shrink: 0; }
.mark i { display: block; height: 4px; width: 100%; border-radius: 2px; background: linear-gradient(90deg, var(--ok-bar) 0 62%, color-mix(in srgb, var(--on-primary) 28%, transparent) 62%); }
h1 { font-size: 22px; margin: 0 0 4px; }
.sub { color: var(--text-soft); font-size: 14px; margin: 0 0 18px; }
.mono { font-family: var(--mono); font-variant-numeric: tabular-nums; }
.storage { display: flex; align-items: flex-start; gap: 11px; padding: 13px; margin-bottom: 18px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2); }
.storage-icon { width: 20px; height: 20px; flex: 0 0 20px; color: var(--primary); }
.storage-icon :deep(svg) { width: 20px; height: 20px; }
.storage b { font-size: 14px; }
.storage p { color: var(--text-soft); font-size: 13px; line-height: 1.5; margin: 3px 0 0; }
.tip { display: flex; gap: 9px; align-items: flex-start; font-size: 13px; line-height: 1.55; color: var(--text-soft); background: var(--surface-2); padding: 10px 12px; border-radius: var(--radius-sm); margin: 16px 0 0; }
.tip-i { display: inline-flex; width: 16px; height: 16px; margin-top: 2px; flex-shrink: 0; color: var(--text-soft); }
.tip-i :deep(svg) { width: 16px; height: 16px; }
.note { font-size: 13px; line-height: 1.5; margin: 12px 0 0; padding: 8px 11px; border-radius: var(--radius-xs); overflow-wrap: anywhere; }
.note.ok { background: var(--ok-soft); color: var(--ok); }
.note.late { background: var(--late-soft); color: var(--late); }
.acts { display: flex; gap: 10px; margin-top: 20px; }
.acts .btn { flex: 1; min-height: 42px; }
.lang { display: flex; justify-content: center; gap: 2px; margin-top: 16px; }
.lang button { border: 0; background: transparent; font: inherit; font-size: 13px; color: var(--text-soft); min-height: 40px; min-width: 44px; padding: 0 10px; border-radius: var(--radius-xs); cursor: pointer; transition: background-color .15s, color .15s; }
.lang button:hover { color: var(--text); background: var(--surface-2); }
.lang button.on { color: var(--text); font-weight: 650; }
@media (max-width: 720px) { .setup { align-items: flex-start; padding: 20px 16px calc(20px + env(safe-area-inset-bottom)); } .setup-card { padding: 20px 18px; } .acts { flex-direction: column-reverse; } }
</style>
