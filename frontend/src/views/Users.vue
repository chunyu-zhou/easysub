<template>
  <div class="users-page">
    <div class="page-head">
      <h1>{{ t('admin.title') }}</h1>
      <button class="btn" @click="openNew"><span class="bi" v-html="icon('plus')"></span>{{ t('admin.createUser') }}</button>
    </div>

    <div class="seg tabs">
      <button :class="{ on: tab === 'pending' }" @click="tab = 'pending'">
        {{ t('admin.pendingTab', { n: pendingUsers.length }) }}
      </button>
      <button :class="{ on: tab === 'all' }" @click="tab = 'all'">{{ t('admin.allTab') }}</button>
    </div>

    <div class="card list-card">
      <div class="table-wrap">
        <table class="u-table">
          <thead>
            <tr>
              <th>{{ t('admin.username') }}</th>
              <th>{{ t('admin.email') }}</th>
              <th>{{ t('admin.role') }}</th>
              <th>{{ t('admin.status') }}</th>
              <th class="num">{{ t('admin.subs') }}</th>
              <th>{{ t('admin.created') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in shown" :key="u.id">
              <td class="c-name">{{ u.username }}</td>
              <td class="c-email" :data-label="t('admin.email')">
                <span class="email">{{ u.email }}</span>
                <span v-if="!u.email_verified" class="tag late unv" :title="t('admin.emailUnverified')"
                  :aria-label="t('admin.emailUnverified')"><span class="ti" v-html="icon('mail')"></span>!</span>
              </td>
              <td :data-label="t('admin.role')">
                <span class="tag" :class="{ adm: u.is_admin }">
                  {{ u.is_admin ? t('admin.admin') : t('admin.user') }}
                </span>
              </td>
              <td :data-label="t('admin.status')">
                <span v-if="!u.is_approved" class="tag soon">{{ t('admin.pending') }}</span>
                <span v-else class="tag" :class="u.is_active ? 'ok' : 'late'">
                  {{ u.is_active ? t('admin.active') : t('admin.disabled') }}
                </span>
              </td>
              <td class="num" :data-label="t('admin.subs')">{{ u.subscription_count }}</td>
              <td class="c-date" :data-label="t('admin.created')">{{ fmt(u.created_at) }}</td>
              <td class="c-acts">
                <div class="acts">
                  <button v-if="!u.is_approved" class="btn sm" @click="approve(u)">
                    <span class="bi" v-html="icon('check')"></span>{{ t('admin.approve') }}
                  </button>
                  <button class="btn sm ghost" @click="toggleAdmin(u)">
                    {{ u.is_admin ? t('admin.revokeAdmin') : t('admin.makeAdmin') }}
                  </button>
                  <button class="btn sm ghost" @click="toggleActive(u)">
                    {{ u.is_active ? t('admin.disable') : t('admin.enable') }}
                  </button>
                  <button class="btn sm ghost" @click="resetPwd(u)">{{ t('admin.resetPwd') }}</button>
                  <button class="btn sm danger ghost" @click="remove(u)">
                    <span class="bi" v-html="icon('trash')"></span>{{ t('sub.delete') }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="!shown.length" class="empty">
        <span class="empty-i" v-html="icon('users')"></span>
        <b>{{ tab === 'pending' ? t('admin.noPending') : t('reports.empty') }}</b>
      </div>
      <p v-if="err" class="note late" role="alert">{{ err }}</p>
    </div>

    <div v-if="showForm" class="modal-mask" @click.self="showForm = false">
      <div class="modal" style="width:420px">
        <h3>{{ t('admin.createUser') }}</h3>
        <label for="nu-name">{{ t('admin.username') }}</label>
        <input id="nu-name" v-model="form.username" autocomplete="off" />
        <label for="nu-email">{{ t('admin.email') }}</label>
        <input id="nu-email" v-model="form.email" type="email" autocomplete="off" />
        <label for="nu-pwd">{{ t('admin.password') }}</label>
        <input id="nu-pwd" v-model="form.password" type="password" autocomplete="new-password" />
        <label class="rb">
          <input type="checkbox" v-model="form.is_admin" /> {{ t('admin.admin') }}
        </label>
        <p v-if="formErr" class="note late m" role="alert">{{ formErr }}</p>
        <div class="row m-acts">
          <button class="btn ghost" @click="showForm = false">{{ t('admin.cancel') }}</button>
          <button class="btn" @click="create">{{ t('admin.create') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { icon } from '../icons'
import api from '../api'

const { t } = useI18n()
const users = ref([])
const err = ref('')
const tab = ref('pending')
const showForm = ref(false)
const formErr = ref('')
const form = ref({ username: '', email: '', password: '', is_admin: false })

function fmt(s) { return s ? new Date(s).toLocaleDateString() : '' }

const pendingUsers = computed(() => users.value.filter((u) => !u.is_approved))
const shown = computed(() => (tab.value === 'pending' ? pendingUsers.value : users.value))

async function load() {
  err.value = ''
  try {
    users.value = (await api.get('/api/admin/users')).data
    // 没有待审核用户时默认展示全部，避免空白
    if (tab.value === 'pending' && !pendingUsers.value.length) tab.value = 'all'
  } catch (e) { err.value = e.response?.data?.detail || 'Error' }
}

function openNew() {
  form.value = { username: '', email: '', password: '', is_admin: false }
  formErr.value = ''; showForm.value = true
}
async function create() {
  formErr.value = ''
  try {
    await api.post('/api/admin/users', form.value)
    showForm.value = false; load()
  } catch (e) { formErr.value = e.response?.data?.detail || 'Error' }
}

async function patch(u, body) {
  err.value = ''
  try { await api.patch(`/api/admin/users/${u.id}`, body); load() }
  catch (e) { err.value = e.response?.data?.detail || 'Error' }
}
const approve = (u) => patch(u, { is_approved: true })
const toggleAdmin = (u) => patch(u, { is_admin: !u.is_admin })
const toggleActive = (u) => patch(u, { is_active: !u.is_active })
function resetPwd(u) {
  const pwd = prompt(t('admin.resetPwdPrompt'))
  if (pwd) patch(u, { password: pwd })
}
async function remove(u) {
  if (!confirm(t('admin.confirmDelete'))) return
  err.value = ''
  try { await api.delete(`/api/admin/users/${u.id}`); load() }
  catch (e) { err.value = e.response?.data?.detail || 'Error' }
}

onMounted(load)
</script>

<style scoped>
.users-page { min-width: 0; }
.bi { display: inline-flex; }
.tabs { margin-bottom: 14px; max-width: 100%; }
.list-card { padding: 4px 0; }

.u-table th, .u-table td { padding: 11px 14px; vertical-align: middle; }
.u-table th:first-child, .u-table td:first-child { padding-left: 20px; }
.u-table th:last-child, .u-table td:last-child { padding-right: 20px; }
.u-table .num { text-align: right; }
.c-name { font-weight: 600; overflow-wrap: anywhere; }
.c-email { color: var(--text-soft); }
.email { overflow-wrap: anywhere; }
.c-date { color: var(--text-soft); white-space: nowrap; }
.unv { margin-left: 6px; padding: 0 7px; gap: 2px; vertical-align: 1px; }
.ti { display: inline-flex; }
.ti :deep(svg) { width: 12px; height: 12px; }
.tag.adm { background: var(--primary-soft); color: var(--text); }
.acts { display: flex; gap: 6px; flex-wrap: wrap; }

.empty-i { display: inline-flex; color: var(--text-faint); }
.note { font-size: 13px; line-height: 1.5; margin: 8px 20px 14px; padding: 8px 11px; border-radius: var(--radius-xs);
  overflow-wrap: anywhere; }
.note.m { margin: 12px 0 0; }
.note.late { background: var(--late-soft); color: var(--late); }

.rb { display: flex; align-items: center; gap: 8px; width: auto; margin: 14px 0 0; min-height: 40px;
  color: var(--text); cursor: pointer; }
.rb input { width: 16px; }
.m-acts { justify-content: flex-end; margin-top: 16px; }

/* 手机端：每个用户一块，操作换行 */
@media (max-width: 720px) {
  .list-card { padding: 0; }
  .table-wrap { overflow: visible; }
  .u-table, .u-table tbody, .u-table tr, .u-table td { display: block; }
  .u-table thead { display: none; }
  .u-table tr { padding: 14px 14px 12px; border-bottom: 1px solid var(--border); }
  .u-table tr:last-child { border-bottom: 0; }
  .u-table tbody tr:hover { background: transparent; }
  .u-table td, .u-table td:first-child, .u-table td:last-child { border: 0; padding: 3px 0; }
  .u-table .num { text-align: left; }
  .c-name { font-size: 16px; padding-bottom: 6px !important; }
  .u-table td[data-label] { display: flex; align-items: center; gap: 10px; font-size: 13.5px; min-width: 0; }
  .u-table td[data-label]::before { content: attr(data-label); flex: 0 0 84px; font-size: 12.5px;
    color: var(--text-soft); font-weight: 500; }
  .c-email { flex-wrap: wrap; }
  .c-email .email { min-width: 0; flex: 0 1 auto; }
  .unv { margin-left: 0; }
  .c-acts { padding-top: 10px !important; }
  .acts .btn { min-height: 40px; flex: 1 1 auto; }
  .tabs { display: flex; }
  .tabs button { flex: 1; min-height: 40px; }
  .m-acts .btn { flex: 1; }
}
</style>
