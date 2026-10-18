<template>
  <div class="settings">
    <header class="page-head">
      <h1>{{ t('settings.title') }}</h1>
    </header>

    <!-- 分区导航：长页面拆成几组，切换时保留各分区状态（v-show） -->
    <nav class="set-tabs" role="tablist" :aria-label="t('settings.title')">
      <button v-for="tb in tabs" :key="tb.key" type="button" role="tab" class="set-tab"
              :class="{ on: tab === tb.key }" :aria-selected="tab === tb.key" @click="setTab(tb.key)">
        <span class="hi" v-html="icon(tb.icon)"></span><span>{{ t('settings.tab.' + tb.key) }}</span>
      </button>
    </nav>

    <!-- ===================== 账户与安全 ===================== -->
    <div v-show="tab === 'account'" class="set-grid" role="tabpanel">
      <!-- 账号与密码 -->
      <section class="card sect">
        <h3 class="sh"><span class="hi" v-html="icon('users')"></span>{{ t('account.title') }}</h3>
        <div class="pair">
          <div>
            <label>{{ t('account.username') }}</label>
            <input v-model="acc.username" autocomplete="username" />
          </div>
          <div>
            <label>{{ t('account.email') }}</label>
            <input v-model="acc.email" type="email" autocomplete="email" />
          </div>
        </div>
        <div class="actions">
          <button class="btn ghost sm" @click="saveAccount">{{ t('account.saveAccount') }}</button>
        </div>
        <hr />
        <div class="pair">
          <div>
            <label>{{ t('account.oldPwd') }}</label>
            <input v-model="pwd.old_password" type="password" autocomplete="current-password" />
          </div>
          <div>
            <label>{{ t('account.newPwd') }}</label>
            <input v-model="pwd.new_password" type="password" autocomplete="new-password" />
          </div>
        </div>
        <div class="actions">
          <button class="btn ghost sm" @click="changePwd">{{ t('account.changePwd') }}</button>
        </div>
        <p v-if="accMsg" class="msg" :class="accOk ? 'ok' : 'err'">
          <span class="mi" v-html="icon(accOk ? 'check' : 'alert')"></span><span>{{ accMsg }}</span>
        </p>
      </section>

      <!-- 外观与偏好 -->
      <section class="card sect">
        <h3 class="sh"><span class="hi" v-html="icon('globe')"></span>{{ t('settings.theme') }} / {{ t('settings.language') }}</h3>
        <div class="pair">
          <div>
            <label>{{ t('settings.language') }}</label>
            <select v-model="locale" @change="changeLocale">
              <option value="zh">中文</option>
              <option value="en">English</option>
              <option value="ru">Русский</option>
            </select>
          </div>
          <div>
            <label>{{ t('settings.baseCurrency') }}</label>
            <select v-model="baseCurrency" @change="changeCurrency">
              <option v-for="c in currencies" :key="c.code" :value="c.code">{{ curLabel(c, locale) }}</option>
            </select>
          </div>
        </div>
        <label>{{ t('settings.theme') }}</label>
        <div class="theme-picker" role="radiogroup" :aria-label="t('settings.theme')">
          <button v-for="th in themes" :key="th.v" type="button" class="th" :class="{ on: theme === th.v }"
                  role="radio" :aria-checked="theme === th.v" :title="themeName(th)"
                  @click="theme = th.v; changeTheme()">
            <span class="sw" :style="{ background: th.bg }"><i :style="{ background: th.c }"></i></span>
            <span class="th-n">{{ themeName(th) }}</span>
          </button>
        </div>
      </section>

      <!-- 提醒与预算 -->
      <section class="card sect">
        <h3 class="sh"><span class="hi" v-html="icon('clock')"></span>{{ t('remind.title') }}</h3>
        <div class="trio">
          <div>
            <label>{{ t('remind.budget') }}（{{ baseCurrency }}）</label>
            <input v-model.number="prefs.monthly_budget" type="number" min="0" :placeholder="t('remind.budgetPh')" />
          </div>
          <div>
            <label>{{ t('remind.quietStart') }}</label>
            <input v-model="prefs.quiet_start" type="time" />
          </div>
          <div>
            <label>{{ t('remind.quietEnd') }}</label>
            <input v-model="prefs.quiet_end" type="time" />
          </div>
        </div>
        <p class="muted hint">{{ t('remind.quietHint') }}</p>
        <div class="toggle-row">
          <label class="switch"><input type="checkbox" v-model="prefs.digest_enabled" /> <span>{{ t('remind.digest') }}</span></label>
          <select v-if="prefs.digest_enabled" v-model.number="prefs.digest_weekday" class="auto-w">
            <option v-for="(d, i) in weekdays" :key="i" :value="i">{{ d }}</option>
          </select>
        </div>
        <div class="actions">
          <button class="btn" @click="savePrefs">{{ t('settings.save') }}</button>
          <p v-if="prefsMsg" class="msg ok inline">
            <span class="mi" v-html="icon('check')"></span><span>{{ prefsMsg }}</span>
          </p>
        </div>
      </section>

      <!-- 安全：两步验证 + API Token -->
      <section class="card sect">
        <h3 class="sh"><span class="hi" v-html="icon('check')"></span>{{ t('sec.title') }}</h3>
        <div class="sec-row">
          <div class="sec-l">
            <b>{{ t('sec.twofa') }}</b>
            <span class="tag" :class="{ ok: auth.user?.totp_enabled }">{{ auth.user?.totp_enabled ? t('sec.on') : t('sec.off') }}</span>
          </div>
          <button v-if="!auth.user?.totp_enabled && !twofa.qr" class="btn ghost sm" @click="twofaSetup">{{ t('sec.enable') }}</button>
          <button v-else-if="auth.user?.totp_enabled" class="btn ghost sm" @click="twofaDisable">{{ t('sec.disable') }}</button>
        </div>
        <div v-if="twofa.qr && !auth.user?.totp_enabled" class="twofa-box">
          <img :src="twofa.qr" class="qr" alt="QR" />
          <div class="twofa-r">
            <p class="muted hint top">{{ t('sec.scanTip') }}</p>
            <code class="secret">{{ twofa.secret }}</code>
            <div class="inline-form">
              <input v-model="twofa.code" :placeholder="t('auth.otpPh')" maxlength="6" inputmode="numeric" class="otp" />
              <button class="btn sm" @click="twofaEnable">{{ t('sec.confirmEnable') }}</button>
            </div>
          </div>
        </div>
        <p v-if="secMsg" class="msg" :class="secOk ? 'ok' : 'err'">
          <span class="mi" v-html="icon(secOk ? 'check' : 'alert')"></span><span>{{ secMsg }}</span>
        </p>

        <hr />
        <div class="sec-row">
          <b>{{ t('sec.apiTokens') }}</b>
          <div class="inline-form">
            <input v-model="newTokenName" :placeholder="t('sec.tokenName')" class="tk-name" @keyup.enter="createToken" />
            <button class="btn ghost sm icon" :title="t('sec.tokenName')" :aria-label="t('sec.tokenName')" @click="createToken">
              <span v-html="icon('plus')"></span>
            </button>
          </div>
        </div>
        <div v-if="createdToken" class="token-once">
          <p class="msg ok"><span class="mi" v-html="icon('check')"></span><span>{{ t('sec.tokenOnce') }}</span></p>
          <code class="code">{{ createdToken }}</code>
        </div>
        <ul v-if="tokens.length" class="item-list">
          <li v-for="tk in tokens" :key="tk.id">
            <div class="il-main">
              <b>{{ tk.name }}</b>
              <span class="muted">{{ tk.masked }} · {{ tk.last_used_at ? fmtTime(tk.last_used_at) : t('sec.neverUsed') }}</span>
            </div>
            <button class="btn ghost danger sm icon" :aria-label="t('sec.revoke')" :title="t('sec.revoke')" @click="revokeToken(tk.id)">
              <span v-html="icon('x')"></span>
            </button>
          </li>
        </ul>
      </section>

      <!-- API 与 MCP：复用上方 API Token -->
      <section class="card sect span-all">
        <h3 class="sh"><span class="hi" v-html="icon('sparkle')"></span>{{ t('mcp.title') }}</h3>
        <p class="muted desc">{{ t('mcp.tip') }}</p>
        <div class="mcp-grid">
          <div>
            <label>{{ t('mcp.restBase') }}</label>
            <div class="copy-row">
              <code class="code">{{ apiBase }}</code>
              <button class="btn ghost sm icon" :title="t('cal.copy')" :aria-label="t('cal.copy')" @click="copyText(apiBase)"><span v-html="icon('note')"></span></button>
            </div>
            <p class="hint"><a :href="sys?.api_docs || '/api/docs'" target="_blank" rel="noopener">{{ t('mcp.docs') }}</a></p>
            <p v-if="sys && sys.mcp_enabled === false" class="muted hint">{{ t('mcp.disabled') }}</p>
            <template v-else>
              <label>{{ t('mcp.endpoint') }}</label>
              <div class="copy-row">
                <code class="code">{{ mcpUrl }}</code>
                <button class="btn ghost sm icon" :title="t('cal.copy')" :aria-label="t('cal.copy')" @click="copyText(mcpUrl)"><span v-html="icon('note')"></span></button>
              </div>
              <p class="muted hint">{{ t('mcp.transport') }} · <span class="mono-s">Authorization: Bearer &lt;API Token&gt;</span></p>
              <p v-if="!createdToken" class="muted hint">{{ t('mcp.tokenHint') }}</p>
            </template>
          </div>
          <div v-if="!(sys && sys.mcp_enabled === false)">
            <label>Claude Code</label>
            <div class="copy-row">
              <code class="code">{{ mcpClaudeCmd }}</code>
              <button class="btn ghost sm icon" :title="t('cal.copy')" :aria-label="t('cal.copy')" @click="copyText(mcpClaudeCmd)"><span v-html="icon('note')"></span></button>
            </div>
            <label>{{ t('mcp.jsonCfg') }}</label>
            <div class="copy-row">
              <pre class="code pre">{{ mcpJson }}</pre>
              <button class="btn ghost sm icon" :title="t('cal.copy')" :aria-label="t('cal.copy')" @click="copyText(mcpJson)"><span v-html="icon('note')"></span></button>
            </div>
          </div>
        </div>
        <p v-if="copyMsg" class="msg ok"><span class="mi" v-html="icon('check')"></span><span>{{ copyMsg }}</span></p>
      </section>
    </div>

    <!-- ===================== 通知 ===================== -->
    <!-- 包一层边界：某个渠道面板万一渲染出错，只坏这一块并显示原因，不会让整栏静默消失。 -->
    <div v-show="tab === 'notify'" role="tabpanel">
      <ErrorBoundary><NotifyChannels /></ErrorBoundary>
    </div>

    <!-- ===================== 货币与付款 ===================== -->
    <div v-show="tab === 'money'" class="set-grid" role="tabpanel">
      <!-- 货币与汇率（合并原「常用货币汇率」与「货币管理」） -->
      <section class="card sect span-all">
        <div class="sh-row">
          <h3 class="sh"><span class="hi" v-html="icon('wallet')"></span>{{ t('cur.title') }}</h3>
          <div class="head-acts">
            <button class="btn ghost sm" @click="refreshRates"><span v-html="icon('renew')"></span>{{ t('settings.refreshRates') }}</button>
            <button class="btn ghost sm" @click="restoreCurrencies">{{ t('cur.restore') }}</button>
            <button class="btn sm" @click="openCurAdd"><span v-html="icon('plus')"></span>{{ t('cur.addBtn') }}</button>
          </div>
        </div>
        <p class="muted desc">
          {{ t('cur.tip', { base: rates.base }) }}
          <span v-if="rates.updated_at"> · {{ t('settings.updatedAt') }} {{ fmtTime(rates.updated_at) }}</span>
          <span v-if="rateMsg" class="rate-msg"> · {{ rateMsg }}</span>
        </p>

        <div v-if="currencies.length" class="cur-grid">
          <div v-for="(c, i) in currencies" :key="c.code" class="cur-item">
            <div class="cur-badge"><span class="mono-s">{{ c.code }}</span><span class="cur-sym">{{ c.symbol }}</span></div>
            <div class="cur-main">
              <div class="cur-name">
                <b>{{ c.name_zh || c.name }}</b>
                <span v-if="c.code === baseCurrency" class="tag ok">{{ t('cur.isBase') }}</span>
                <span v-if="c.is_custom" class="tag">{{ t('cur.custom') }}</span>
              </div>
              <div v-if="c.name_zh && c.name && c.name !== c.name_zh" class="muted cur-sub">{{ c.name }}</div>
              <div class="cur-rate tabular">{{ rateText(c) }}<span v-if="c.manual_rate != null" class="tag manual"
                :title="t('cur.manualShow', { base: sysBase || rates.base, rate: c.manual_rate, code: c.code })">{{ t('cur.manualTag') }}</span></div>
            </div>
            <div class="il-acts">
              <span class="cur-move">
                <button class="btn ghost sm icon xs" :disabled="i === 0" :title="t('cur.up')" :aria-label="t('cur.up')" @click="moveCurrency(i, -1)"><span class="rot-up" v-html="icon('chevron')"></span></button>
                <button class="btn ghost sm icon xs" :disabled="i === currencies.length - 1" :title="t('cur.down')" :aria-label="t('cur.down')" @click="moveCurrency(i, 1)"><span class="rot-down" v-html="icon('chevron')"></span></button>
              </span>
              <button class="btn ghost sm icon" :title="t('sub.edit')" :aria-label="t('sub.edit')" @click="openCurEdit(c)"><span v-html="icon('edit')"></span></button>
              <button class="btn ghost danger sm icon" :title="t('sub.delete')" :aria-label="t('sub.delete')" @click="deleteCurrency(c)"><span v-html="icon('trash')"></span></button>
            </div>
          </div>
        </div>
        <p v-else class="muted hint">{{ t('settings.noRates') }}</p>
        <p v-if="curMsg" class="msg" :class="curOk ? 'ok' : 'err'">
          <span class="mi" v-html="icon(curOk ? 'check' : 'alert')"></span><span>{{ curMsg }}</span>
        </p>
      </section>

      <!-- 付款方式 -->
      <section class="card sect span-all">
        <div class="sh-row">
          <h3 class="sh"><span class="hi" v-html="icon('card')"></span>{{ t('pm.title') }}</h3>
          <div class="head-acts">
            <button class="btn ghost sm" @click="restoreMethods">{{ t('pm.restore') }}</button>
            <button class="btn sm" @click="newMethod"><span v-html="icon('plus')"></span>{{ t('pm.add') }}</button>
          </div>
        </div>
        <p class="muted desc">{{ t('pm.tip') }}</p>
        <div v-if="pms.length" class="pm-grid">
          <div v-for="(p, i) in pms" :key="p.id" class="pm-item">
            <SubIcon :icon="p.icon" :name="p.name" :size="38" />
            <div class="pm-main">
              <b class="pm-name">{{ p.name }}</b>
              <span v-if="p.account" class="pm-acc">
                <code class="mono-s">{{ p.account }}</code>
                <button class="btn ghost sm icon xs" :title="t('cal.copy')" :aria-label="t('cal.copy')" @click="copyText(p.account, 'pm')"><span v-html="icon('note')"></span></button>
              </span>
              <span v-if="p.note" class="muted pm-note">{{ p.note }}</span>
            </div>
            <div class="il-acts col">
              <button class="btn ghost sm icon" :title="t('sub.edit')" :aria-label="t('sub.edit')" @click="editMethod(p)"><span v-html="icon('edit')"></span></button>
              <button class="btn ghost danger sm icon" :title="t('sub.delete')" :aria-label="t('sub.delete')" @click="deleteMethod(p)"><span v-html="icon('trash')"></span></button>
            </div>
            <div class="pm-move">
              <button class="btn ghost sm icon xs" :disabled="i === 0" :title="t('cur.up')" :aria-label="t('cur.up')" @click="moveMethod(i, -1)"><span class="rot-left" v-html="icon('chevron')"></span></button>
              <button class="btn ghost sm icon xs" :disabled="i === pms.length - 1" :title="t('cur.down')" :aria-label="t('cur.down')" @click="moveMethod(i, 1)"><span v-html="icon('chevron')"></span></button>
            </div>
          </div>
        </div>
        <p v-else class="muted hint">{{ t('pm.none') }}</p>
        <p v-if="pmMsg" class="msg" :class="pmOk ? 'ok' : 'err'">
          <span class="mi" v-html="icon(pmOk ? 'check' : 'alert')"></span><span>{{ pmMsg }}</span>
        </p>
      </section>
    </div>

    <!-- ===================== 服务库 ===================== -->
    <div v-show="tab === 'services'" role="tabpanel">
      <section class="card sect">
        <div class="sh-row">
          <h3 class="sh"><span class="hi" v-html="icon('library')"></span>{{ t('svc.title') }}</h3>
          <div class="head-acts">
            <button class="btn ghost sm" @click="restoreServices">{{ t('svc.restore') }}</button>
            <button class="btn sm" @click="openSvc()"><span v-html="icon('plus')"></span>{{ t('svc.add') }}</button>
          </div>
        </div>
        <p class="muted desc">{{ t('svc.tip') }}</p>
        <div class="svc-tools">
          <input v-model="svcQ" type="search" :placeholder="t('svc.searchPh')" />
          <select v-model="svcCat" class="auto-w">
            <option value="">{{ t('svc.allCats') }}</option>
            <option v-for="c in svcCats" :key="c.key" :value="c.key">{{ c.label }}</option>
          </select>
          <span class="muted svc-count">{{ t('svc.count', { n: svcFiltered.length }) }}</span>
        </div>
        <p v-if="svcMsg" class="msg" :class="svcOk ? 'ok' : 'err'">
          <span class="mi" v-html="icon(svcOk ? 'check' : 'alert')"></span><span>{{ svcMsg }}</span>
        </p>
        <div v-for="g in svcGroups" :key="g.key" class="svc-group">
          <div class="svc-group-t">{{ g.label }} <span class="tag">{{ g.items.length }}</span></div>
          <div class="svc-grid">
            <div v-for="s in g.items" :key="s.id" class="svc-item">
              <SubIcon :icon="s.icon" :name="s.name" :size="32" />
              <div class="svc-main">
                <b :title="s.name">{{ s.name }}</b>
                <span class="muted">{{ s.domain || '—' }}</span>
              </div>
              <div class="il-acts">
                <button class="btn ghost sm icon" :title="t('sub.edit')" :aria-label="t('sub.edit')" @click="openSvc(s)"><span v-html="icon('edit')"></span></button>
                <button class="btn ghost danger sm icon" :title="t('sub.delete')" :aria-label="t('sub.delete')" @click="deleteSvc(s)"><span v-html="icon('trash')"></span></button>
              </div>
            </div>
          </div>
        </div>
        <p v-if="!svcGroups.length" class="muted hint">{{ t('svc.none') }}</p>
      </section>
    </div>

    <!-- ===================== 数据与备份 ===================== -->
    <div v-show="tab === 'data'" class="set-grid" role="tabpanel">
      <!-- 数据备份与恢复 -->
      <section class="card sect">
        <h3 class="sh"><span class="hi" v-html="icon('package')"></span>{{ t('backup.title') }}</h3>
        <p class="muted desc">{{ t('backup.tip') }}</p>
        <div class="actions">
          <button class="btn ghost" @click="exportData"><span v-html="icon('download')"></span>{{ t('backup.export') }}</button>
          <label class="btn ghost file-btn"><span v-html="icon('upload')"></span>{{ t('backup.import') }}
            <input type="file" accept="application/json,.json" hidden @change="importData" />
          </label>
        </div>
        <div class="toggle-row">
          <label class="switch"><input type="checkbox" v-model="importReplace" /> <span>{{ t('backup.replace') }}</span></label>
        </div>
        <p v-if="backupMsg" class="msg" :class="backupOk ? 'ok' : 'err'">
          <span class="mi" v-html="icon(backupOk ? 'check' : 'alert')"></span><span>{{ backupMsg }}</span>
        </p>
      </section>

      <!-- 日历订阅 (.ics) -->
      <section class="card sect">
        <h3 class="sh"><span class="hi" v-html="icon('calendar')"></span>{{ t('cal.title') }}</h3>
        <p class="muted desc">{{ t('cal.tip') }}</p>
        <div class="actions">
          <button class="btn ghost" @click="loadCalUrl"><span v-html="icon('globe')"></span>{{ t('cal.get') }}</button>
          <button v-if="calUrl" class="btn ghost" @click="copyCal"><span v-html="icon('note')"></span>{{ t('cal.copy') }}</button>
          <button v-if="calUrl" class="btn ghost" @click="resetCal"><span v-html="icon('renew')"></span>{{ t('cal.reset') }}</button>
        </div>
        <input v-if="calUrl" :value="calUrl" readonly class="cal-url" @focus="$event.target.select()" />
        <p v-if="calMsg" class="msg neutral">{{ calMsg }}</p>
      </section>

      <!-- WebDAV 备份与恢复 -->
      <section class="card sect span-all">
        <h3 class="sh"><span class="hi" v-html="icon('upload')"></span>{{ t('dav.title') }}</h3>
        <p class="muted desc">{{ t('dav.tip') }}</p>
        <div class="dav-grid">
          <div>
            <label>{{ t('dav.url') }}</label>
            <input v-model="dav.url" placeholder="https://dav.jianguoyun.com/dav/" autocomplete="off" />
            <div class="pair">
              <div><label>{{ t('dav.username') }}</label><input v-model="dav.username" autocomplete="off" /></div>
              <div><label>{{ t('dav.password') }}</label>
                <input v-model="dav.password" type="password" autocomplete="new-password"
                       :placeholder="davHasPwd ? t('dav.pwdKeep') : ''" /></div>
            </div>
            <div class="pair">
              <div><label>{{ t('dav.path') }}</label><input v-model="dav.path" placeholder="easysub" /></div>
              <div><label>{{ t('dav.keep') }}</label><input v-model.number="dav.keep" type="number" min="1" /></div>
            </div>
            <template v-if="auth.user?.is_admin">
              <label>{{ t('dav.scope') }}</label>
              <select v-model="dav.scope">
                <option value="self">{{ t('dav.scopeSelf') }}</option>
                <option value="all">{{ t('dav.scopeAll') }}</option>
              </select>
            </template>
            <div class="toggle-row">
              <label class="switch"><input type="checkbox" v-model="dav.auto_enabled" /> <span>{{ t('dav.auto') }}</span></label>
            </div>
            <div class="actions">
              <button class="btn ghost sm" :disabled="davBusy" @click="testDav"><span v-html="icon('send')"></span>{{ t('dav.test') }}</button>
              <button class="btn sm" :disabled="davBusy" @click="saveDav">{{ t('settings.save') }}</button>
              <button class="btn ghost sm" :disabled="davBusy" @click="backupDav"><span v-html="icon('package')"></span>{{ t('dav.backupNow') }}</button>
            </div>
            <p v-if="davLast" class="hint" :class="davLast.ok ? 'last-ok' : 'last-err'">
              {{ t('dav.last') }} {{ fmtTime(davLast.at) }} · {{ davLast.ok ? t('dav.lastOk') : t('dav.lastFail') }}<span v-if="davLast.file"> · {{ davLast.file }}</span><span v-if="davLast.message"> · {{ davLast.message }}</span>
            </p>
            <p v-if="davMsg" class="msg" :class="davOk ? 'ok' : 'err'">
              <span class="mi" v-html="icon(davOk ? 'check' : 'alert')"></span><span>{{ davMsg }}</span>
            </p>
          </div>

          <div class="dav-files">
            <div class="sh-row">
              <h4 class="sub-h"><span class="hi" v-html="icon('inbox')"></span>{{ t('dav.files') }}</h4>
              <button class="btn ghost sm" :disabled="davBusy" @click="loadDavFiles(true)"><span v-html="icon('renew')"></span>{{ t('dav.refresh') }}</button>
            </div>
            <div class="toggle-row">
              <label class="switch"><input type="checkbox" v-model="davReplace" /> <span>{{ t('dav.replace') }}</span></label>
            </div>
            <ul v-if="davFiles.length" class="item-list">
              <li v-for="f in davFiles" :key="f.name">
                <div class="il-main">
                  <b>{{ f.name }}</b>
                  <span class="muted tabular">{{ humanSize(f.size) }} · {{ fmtTime(f.modified) }}</span>
                </div>
                <div class="il-acts">
                  <button class="btn ghost sm" :disabled="davBusy" @click="restoreDav(f)">{{ t('dav.restore') }}</button>
                  <button class="btn ghost danger sm icon" :disabled="davBusy" :title="t('sub.delete')" :aria-label="t('sub.delete')" @click="deleteDavFile(f)"><span v-html="icon('trash')"></span></button>
                </div>
              </li>
            </ul>
            <p v-else class="muted hint">{{ t('dav.noFiles') }}</p>
          </div>
        </div>
      </section>

      <!-- 管理员：整站备份与恢复 -->
      <section class="card sect span-all" v-if="auth.user?.is_admin">
        <h3 class="sh"><span class="hi" v-html="icon('library')"></span>{{ t('backupAll.title') }}</h3>
        <div class="dav-grid">
          <div>
            <p class="muted desc">{{ t('backupAll.tip') }}</p>
            <div class="actions">
              <button class="btn ghost" @click="exportAll"><span v-html="icon('download')"></span>{{ t('backupAll.export') }}</button>
              <label class="btn ghost file-btn"><span v-html="icon('upload')"></span>{{ t('backupAll.import') }}
                <input type="file" accept="application/json,.json" hidden @change="importAll" />
              </label>
            </div>
            <div class="toggle-row">
              <label class="switch"><input type="checkbox" v-model="importAllReplace" /> <span>{{ t('backupAll.replace') }}</span></label>
            </div>
            <p v-if="backupAllMsg" class="msg" :class="backupAllOk ? 'ok' : 'err'">
              <span class="mi" v-html="icon(backupAllOk ? 'check' : 'alert')"></span><span>{{ backupAllMsg }}</span>
            </p>
          </div>
          <div>
            <h4 class="sub-h"><span class="hi" v-html="icon('clock')"></span>{{ t('autobk.title') }}</h4>
            <p class="muted desc">{{ t('autobk.tip') }}</p>
            <div class="actions">
              <button class="btn ghost sm" @click="runAutoBackup"><span v-html="icon('package')"></span>{{ t('autobk.run') }}</button>
              <span class="muted hint" v-if="autobk">{{ t('autobk.keep', { n: autobk.keep }) }}</span>
            </div>
            <ul v-if="autobk && autobk.files.length" class="item-list">
              <li v-for="f in autobk.files" :key="f.name">
                <div class="il-main">
                  <b>{{ f.name }}</b>
                  <span class="muted tabular">{{ humanSize(f.size) }} · {{ fmtTime(f.modified) }}</span>
                </div>
              </li>
            </ul>
            <p v-else-if="autobk" class="muted hint">{{ t('autobk.none') }}</p>
          </div>
        </div>
      </section>
    </div>

    <!-- ===================== 系统 ===================== -->
    <div v-show="tab === 'system'" class="set-grid" role="tabpanel">
      <!-- 版本与更新 -->
      <section class="card sect">
        <h3 class="sh"><span class="hi" v-html="icon('renew')"></span>{{ t('upd.title') }}</h3>
        <div class="ver-row">
          <span class="ver">{{ t('sys.version') }} <b class="tabular">v{{ sys?.version }}</b></span>
          <button class="btn ghost sm" :disabled="updChecking" @click="checkUpdate(true)">
            <span v-html="icon('renew')"></span>{{ t('upd.check') }}
          </button>
        </div>
        <template v-if="upd && !upd.error">
          <p v-if="upd.update_available" class="msg ok">
            <span class="mi" v-html="icon('sparkle')"></span><b>{{ t('upd.newVersion', { v: upd.latest }) }}</b>
          </p>
          <p v-else-if="upd.latest" class="msg ok">
            <span class="mi" v-html="icon('check')"></span><span>{{ t('upd.isLatest') }}</span>
          </p>
        </template>
        <p v-else-if="upd && upd.error" class="msg neutral">
          <span class="mi" v-html="icon('alert')"></span><span>{{ t('upd.failed') }}</span>
        </p>
        <div v-if="upd && upd.update_available" class="upd-box">
          <p class="upd-how">{{ t('upd.howto') }}
            <a :href="upd.release_url" target="_blank" rel="noopener">{{ t('upd.view') }}</a></p>
          <code class="code">docker compose pull &amp;&amp; docker compose up -d</code>
          <p class="muted hint">{{ t('upd.backupTip') }}</p>
        </div>
      </section>

      <!-- 系统信息 -->
      <section class="card sect">
        <h3 class="sh"><span class="hi" v-html="icon('dashboard')"></span>{{ t('sys.title') }}</h3>
        <dl class="kv" v-if="sys">
          <div><dt>{{ t('sys.version') }}</dt><dd>{{ sys.version }}</dd></div>
          <div><dt>{{ t('sys.dbStatus') }}</dt>
            <dd v-if="sys.db_configured" class="db-ok"><span class="dot ok"></span>{{ t('sys.configured') }}</dd>
            <dd v-else>—</dd></div>
          <div><dt>{{ t('sys.serverTime') }}</dt><dd>{{ sys.server_time }}</dd></div>
          <div><dt>{{ t('sys.timezone') }}</dt><dd>{{ sys.timezone }}</dd></div>
          <div><dt>{{ t('sys.scanTime') }}</dt><dd>{{ sys.reminder_scan_time }}</dd></div>
          <div><dt>{{ t('sys.yourSubs') }}</dt><dd>{{ sys.your_subscriptions }}</dd></div>
          <div v-if="sys.total_users != null"><dt>{{ t('sys.totalUsers') }}</dt><dd>{{ sys.total_users }}</dd></div>
          <div v-if="sys.total_subscriptions != null"><dt>{{ t('sys.totalSubs') }}</dt><dd>{{ sys.total_subscriptions }}</dd></div>
        </dl>
        <p v-else class="muted hint">{{ t('common.loading') }}</p>
      </section>
    </div>

    <!-- ===================== 弹窗：添加 / 编辑货币 ===================== -->
    <div v-if="curModal.open" class="modal-mask" @click.self="curModal.open = false">
      <div class="modal" style="width:520px" role="dialog" aria-modal="true">
        <h3>{{ curModal.mode === 'edit' ? t('cur.editTitle') : t('cur.addTitle') }}</h3>

        <div v-if="curModal.mode !== 'edit'" class="seg modal-seg">
          <button :class="{ on: curModal.mode === 'catalog' }" @click="curModal.mode = 'catalog'; curModal.err = ''; curModal.note = ''">{{ t('cur.tabCatalog') }}</button>
          <button :class="{ on: curModal.mode === 'custom' }" @click="curModal.mode = 'custom'; curModal.err = ''; curModal.note = ''">{{ t('cur.tabCustom') }}</button>
        </div>

        <!-- 从内置目录选择：点一项即添加，可连续添加多个 -->
        <template v-if="curModal.mode === 'catalog'">
          <input v-model="curQ" type="search" :placeholder="t('cur.searchPh')" class="modal-search" />
          <ul v-if="catalogLeft.length" class="item-list pick-list">
            <li v-for="c in catalogLeft" :key="c.code">
              <div class="il-main">
                <b><span class="mono-s">{{ c.code }}</span> <span class="cur-sym">{{ c.symbol }}</span> {{ c.name_zh || c.name }}</b>
                <span class="muted">{{ c.name }}</span>
              </div>
              <button class="btn ghost sm" :disabled="curModal.busy" @click="addCatalogCurrency(c.code)"><span v-html="icon('plus')"></span>{{ t('cur.add') }}</button>
            </li>
          </ul>
          <p v-else class="muted hint">{{ curQ ? t('cur.noMatch') : t('cur.allAdded') }}</p>
        </template>

        <!-- 自定义 / 编辑 -->
        <template v-else>
          <div class="pair">
            <div><label>{{ t('cur.code') }}</label>
              <input v-model="curForm.code" class="mono-s" maxlength="16" placeholder="USDT" :disabled="curModal.mode === 'edit'" /></div>
            <div><label>{{ t('cur.symbol') }}</label><input v-model="curForm.symbol" placeholder="₮" /></div>
          </div>
          <div class="pair">
            <div><label>{{ t('cur.nameZh') }}</label><input v-model="curForm.name_zh" :placeholder="t('cur.nameZhPh')" /></div>
            <div><label>{{ t('cur.nameEn') }}</label><input v-model="curForm.name" :placeholder="t('cur.nameEnPh')" /></div>
          </div>
          <label>{{ t('cur.manualRate', { base: sysBase || rates.base, code: curForm.code.trim().toUpperCase() || 'XXX' }) }}</label>
          <input v-model="curForm.manual_rate" type="number" step="any" min="0" :placeholder="t('cur.manualPh')" />
          <p class="muted hint">{{ t('cur.manualHint') }}</p>
        </template>

        <p v-if="curModal.err" class="msg err"><span class="mi" v-html="icon('alert')"></span><span>{{ curModal.err }}</span></p>
        <p v-else-if="curModal.note" class="msg ok"><span class="mi" v-html="icon('check')"></span><span>{{ curModal.note }}</span></p>
        <div class="modal-acts">
          <button class="btn ghost" @click="curModal.open = false">{{ curModal.mode === 'catalog' ? t('common.close') : t('common.cancel') }}</button>
          <button v-if="curModal.mode !== 'catalog'" class="btn" :disabled="curModal.busy || !curForm.code.trim()" @click="submitCurrency">
            {{ curModal.mode === 'edit' ? t('settings.save') : t('cur.add') }}</button>
        </div>
      </div>
    </div>

    <!-- ===================== 弹窗：添加 / 编辑付款方式 ===================== -->
    <div v-if="pmForm.show" class="modal-mask" @click.self="pmForm.show = false">
      <div class="modal" style="width:500px" role="dialog" aria-modal="true">
        <h3>{{ pmForm.id ? t('pm.edit') : t('pm.add') }}</h3>
        <label>{{ t('pm.name') }}</label>
        <input v-model="pmForm.name" :placeholder="t('pm.namePh')" maxlength="64" />

        <label>{{ t('pm.icon') }}</label>
        <div class="icon-field">
          <SubIcon :icon="pmForm.icon.trim()" :name="pmForm.name" :size="40" />
          <input v-model="pmForm.icon" :placeholder="t('pm.iconPh')" maxlength="512" />
          <button type="button" class="btn ghost sm" :aria-expanded="emojiOpen" @click="emojiOpen = !emojiOpen">
            {{ t('pm.pickIcon') }}<span class="chev" :class="{ open: emojiOpen }" v-html="icon('chevron')"></span></button>
        </div>
        <div v-if="emojiOpen" class="emoji-pop" role="listbox" :aria-label="t('pm.pickIcon')">
          <button v-for="e in emojiChoices" :key="e" type="button" class="emoji-i" role="option"
                  :class="{ on: pmForm.icon.trim() === e }" :aria-selected="pmForm.icon.trim() === e"
                  @click="pmForm.icon = e; emojiOpen = false">{{ e }}</button>
        </div>
        <p class="muted hint">{{ t('pm.iconHint') }}</p>

        <label>{{ t('pm.account') }}</label>
        <input v-model="pmForm.account" class="mono-s" :placeholder="t('pm.accountPh')" autocomplete="off" />
        <label>{{ t('pm.note') }}</label>
        <textarea v-model="pmForm.note" rows="2" :placeholder="t('pm.notePh')"></textarea>

        <p v-if="pmForm.err" class="msg err"><span class="mi" v-html="icon('alert')"></span><span>{{ pmForm.err }}</span></p>
        <div class="modal-acts">
          <button class="btn ghost" @click="pmForm.show = false">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="!pmForm.name.trim()" @click="saveMethod">{{ t('settings.save') }}</button>
        </div>
      </div>
    </div>

    <!-- ===================== 弹窗：添加 / 编辑服务 ===================== -->
    <div v-if="svcForm.show" class="modal-mask" @click.self="svcForm.show = false">
      <div class="modal" style="width:500px" role="dialog" aria-modal="true">
        <h3>{{ svcForm.id ? t('svc.editTitle') : t('svc.addTitle') }}</h3>
        <label>{{ t('svc.name') }}</label>
        <input v-model="svcForm.name" :placeholder="t('svc.namePh')" maxlength="128" />
        <label>{{ t('svc.website') }}</label>
        <input v-model="svcForm.website" :placeholder="t('svc.websitePh')" maxlength="512" autocomplete="off" />
        <label>{{ t('svc.category') }}</label>
        <select v-model="svcForm.category">
          <option v-for="c in svcCats" :key="c.key" :value="c.key">{{ c.label }}</option>
        </select>
        <label>{{ t('svc.icon') }}</label>
        <div class="icon-field">
          <SubIcon :icon="svcForm.icon.trim() || svcForm.autoIcon" :name="svcForm.name" :size="40" />
          <input v-model="svcForm.icon" :placeholder="t('svc.iconPh')" maxlength="512" />
        </div>
        <p v-if="svcForm.err" class="msg err"><span class="mi" v-html="icon('alert')"></span><span>{{ svcForm.err }}</span></p>
        <div class="modal-acts">
          <button class="btn ghost" @click="svcForm.show = false">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="svcForm.busy || !svcForm.name.trim()" @click="saveSvc">{{ t('settings.save') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api from '../api'
import { useAuth } from '../stores/auth'
import NotifyChannels from '../components/NotifyChannels.vue'
import ErrorBoundary from '../components/ErrorBoundary.vue'
import SubIcon from '../components/SubIcon.vue'
import { icon } from '../icons'
import { curLabel } from '../currency'
import { isImg } from '../lifeline'

const { t, te, locale } = useI18n()
const auth = useAuth()
const route = useRoute()
const router = useRouter()

/* ---------- 分区（同步到 URL hash，便于从其它页面直达，如 /settings#services） ---------- */
const tabs = [
  { key: 'account', icon: 'users' },
  { key: 'notify', icon: 'send' },
  { key: 'money', icon: 'wallet' },
  { key: 'services', icon: 'library' },
  { key: 'data', icon: 'package' },
  { key: 'system', icon: 'dashboard' }
]
const tabKeys = tabs.map((x) => x.key)
function tabFromHash(h) { const k = (h || '').replace(/^#/, ''); return tabKeys.includes(k) ? k : '' }
const tab = ref(tabFromHash(route.hash) || 'account')
function setTab(k) {
  tab.value = k
  router.replace({ hash: '#' + k })
}
watch(() => route.hash, (h) => { const k = tabFromHash(h); if (k) tab.value = k })
// 手机上标签栏横向滚动：保证当前标签可见
function revealTab() { document.querySelector('.set-tab.on')?.scrollIntoView({ block: 'nearest', inline: 'nearest' }) }
watch(tab, revealTab, { flush: 'post' })

// 色板预览：bg = 该主题底色，c = 该主题操作色（与 styles/theme.css 对应）
const themes = [
  { v: 'light', k: 'Light', bg: '#f6f7f5', c: '#1b1f24' },
  { v: 'dark', k: 'Dark', bg: '#171a1e', c: '#eceef0' },
  { v: 'ocean', k: 'Ocean', bg: '#f1f6f8', c: '#0b5d78' },
  { v: 'forest', k: 'Forest', bg: '#f2f5f0', c: '#2f5d3a' },
  { v: 'purple', k: 'Purple', bg: '#f5f3f9', c: '#5b3fc4' },
  { v: 'auto', k: 'Auto', bg: 'linear-gradient(135deg,#f6f7f5 50%,#171a1e 50%)', c: '#8a919b' }
]
// themeAuto 尚无词条时回退显示 Auto，避免把 key 原样显示出来
function themeName(th) { const k = 'settings.theme' + th.k; return te(k) ? t(k) : th.k }

const theme = ref(auth.user?.theme || 'light')
const baseCurrency = ref(auth.user?.base_currency || 'CNY')
const currencies = ref([])
const rateMsg = ref('')
const rates = ref({ base: baseCurrency.value, updated_at: null, items: [] })

const acc = reactive({ username: auth.user?.username || '', email: auth.user?.email || '' })
const pwd = reactive({ old_password: '', new_password: '' })
const accMsg = ref('')
const accOk = ref(false)
const sys = ref(null)

const calUrl = ref('')
const calMsg = ref('')
const autobk = ref(null)

const upd = ref(null)
const updChecking = ref(false)
async function checkUpdate(force = false) {
  updChecking.value = true
  try { upd.value = (await api.get('/api/system/version-check', { params: force === true ? { refresh: true } : {} })).data }
  catch { /* ignore */ } finally { updChecking.value = false }
}

const ns = auth.user?.notify_settings || {}
const prefs = reactive({
  monthly_budget: auth.user?.monthly_budget ?? null,
  quiet_start: ns.quiet_start || '',
  quiet_end: ns.quiet_end || '',
  digest_enabled: !!ns.digest_enabled,
  digest_weekday: ns.digest_weekday ?? 0
})
const twofa = reactive({ qr: '', secret: '', code: '' })
const secMsg = ref('')
const secOk = ref(false)
const tokens = ref([])
const newTokenName = ref('')
const createdToken = ref('')

async function twofaSetup() {
  secMsg.value = ''
  try { const { data } = await api.post('/api/me/2fa/setup'); twofa.qr = data.qr; twofa.secret = data.secret }
  catch (e) { secOk.value = false; secMsg.value = e.response?.data?.detail || 'Error' }
}
async function twofaEnable() {
  try {
    await api.post('/api/me/2fa/enable', { code: twofa.code })
    twofa.qr = ''; twofa.secret = ''; twofa.code = ''
    await auth.fetchMe(); secOk.value = true; secMsg.value = t('sec.enabledOk')
  } catch (e) { secOk.value = false; secMsg.value = e.response?.data?.detail || 'Error' }
}
async function twofaDisable() {
  const pw = window.prompt(t('sec.enterPwd'))
  if (!pw) return
  try { await api.post('/api/me/2fa/disable', { password: pw }); await auth.fetchMe(); secOk.value = true; secMsg.value = t('sec.disabledOk') }
  catch (e) { secOk.value = false; secMsg.value = e.response?.data?.detail || 'Error' }
}
async function loadTokens() { try { tokens.value = (await api.get('/api/me/tokens')).data } catch { /* ignore */ } }
async function createToken() {
  if (!newTokenName.value.trim()) return
  try {
    const { data } = await api.post('/api/me/tokens', { name: newTokenName.value.trim() })
    createdToken.value = data.token; newTokenName.value = ''; loadTokens()
  } catch (e) { secOk.value = false; secMsg.value = e.response?.data?.detail || 'Error' }
}
async function revokeToken(id) {
  if (!window.confirm(t('sec.revokeConfirm'))) return
  try { await api.delete(`/api/me/tokens/${id}`); loadTokens() } catch { /* ignore */ }
}

const prefsMsg = ref('')
const weekdays = computed(() => [
  t('wk.mon'), t('wk.tue'), t('wk.wed'), t('wk.thu'), t('wk.fri'), t('wk.sat'), t('wk.sun')
])
async function savePrefs() {
  await auth.updateMe({
    monthly_budget: prefs.monthly_budget || null,
    notify_settings: {
      quiet_start: prefs.quiet_start || null, quiet_end: prefs.quiet_end || null,
      digest_enabled: prefs.digest_enabled, digest_weekday: prefs.digest_weekday
    }
  })
  prefsMsg.value = t('settings.saved')
  setTimeout(() => (prefsMsg.value = ''), 3000)
}

const backupMsg = ref('')
const backupOk = ref(false)
const importReplace = ref(false)

const backupAllMsg = ref('')
const backupAllOk = ref(false)
const importAllReplace = ref(false)

async function exportData() {
  backupMsg.value = ''
  try {
    const { data } = await api.get('/api/backup/export')
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    const stamp = new Date().toISOString().slice(0, 10)
    a.href = url
    a.download = `easysub-backup-${stamp}.json`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    backupOk.value = true
    backupMsg.value = t('backup.exportOk')
  } catch (e) {
    backupOk.value = false
    backupMsg.value = e.response?.data?.detail || 'Error'
  }
}

// 读取备份文件：去掉 BOM 再解析；解析失败单独标记，和服务器报错区分开（debug19）
async function readBackupJson(file) {
  const text = (await file.text()).replace(/^﻿/, '')
  try { return JSON.parse(text) } catch { throw { notJson: true } }
}
function importErrText(err) {
  if (err?.notJson) return t('backup.notJson')
  const d = err?.response?.data?.detail
  if (typeof d === 'string' && d) return d
  if (Array.isArray(d) && d.length) return t('backup.httpFail', { s: err.response.status, m: d[0].msg || '' })
  if (err?.response) return t('backup.httpFail', { s: err.response.status, m: String(err.response.statusText || '') })
  return t('backup.netFail')
}

async function importData(e) {
  const file = e.target.files[0]
  if (!file) return
  backupMsg.value = ''
  if (importReplace.value && !window.confirm(t('backup.replaceConfirm'))) {
    e.target.value = ''
    return
  }
  try {
    const json = await readBackupJson(file)
    const { data } = await api.post('/api/backup/import', { data: json, replace: importReplace.value })
    backupOk.value = true
    backupMsg.value = t('backup.importOk', { n: data.imported })
  } catch (err) {
    backupOk.value = false
    backupMsg.value = importErrText(err)
  } finally {
    e.target.value = ''
  }
}

async function exportAll() {
  backupAllMsg.value = ''
  try {
    const { data } = await api.get('/api/backup/export-all')
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    const stamp = new Date().toISOString().slice(0, 10)
    a.href = url
    a.download = `easysub-full-backup-${stamp}.json`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    backupAllOk.value = true
    backupAllMsg.value = t('backupAll.exportOk', { n: data.users?.length || 0 })
  } catch (e) {
    backupAllOk.value = false
    backupAllMsg.value = e.response?.data?.detail || 'Error'
  }
}

async function importAll(e) {
  const file = e.target.files[0]
  if (!file) return
  backupAllMsg.value = ''
  if (!window.confirm(t(importAllReplace.value ? 'backupAll.replaceConfirm' : 'backupAll.importConfirm'))) {
    e.target.value = ''
    return
  }
  try {
    const json = await readBackupJson(file)
    const { data } = await api.post('/api/backup/import-all', { data: json, replace: importAllReplace.value })
    backupAllOk.value = true
    backupAllMsg.value = t('backupAll.importOk', { users: data.users, created: data.created_users, n: data.imported })
  } catch (err) {
    backupAllOk.value = false
    backupAllMsg.value = importErrText(err)
  } finally {
    e.target.value = ''
  }
}

async function saveAccount() {
  accMsg.value = ''
  try {
    await api.patch('/api/me/account', { username: acc.username, email: acc.email })
    await auth.fetchMe()
    accOk.value = true; accMsg.value = t('account.accountOk')
  } catch (e) { accOk.value = false; accMsg.value = e.response?.data?.detail || 'Error' }
}
async function changePwd() {
  accMsg.value = ''
  try {
    await api.post('/api/me/password', pwd)
    pwd.old_password = ''; pwd.new_password = ''
    accOk.value = true; accMsg.value = t('account.pwdOk')
  } catch (e) { accOk.value = false; accMsg.value = e.response?.data?.detail || 'Error' }
}

async function changeLocale() {
  localStorage.setItem('locale', locale.value)
  await auth.updateMe({ locale: locale.value })
}
async function changeTheme() { await auth.updateMe({ theme: theme.value }) }
async function changeCurrency() {
  await auth.updateMe({ base_currency: baseCurrency.value })
  loadRates()
}

function fmtTime(s) { return s ? new Date(s).toLocaleString() : '' }
async function loadRates() {
  try { rates.value = (await api.get('/api/currencies/rate-table')).data }
  catch { /* ignore */ }
}
async function refreshRates() {
  try {
    await api.post('/api/currencies/rates/refresh')
    rateMsg.value = t('settings.ratesUpdated')
    await loadRates()
  } catch (e) { rateMsg.value = e.response?.data?.detail || 'Error' }
}

async function loadCalUrl(reset = false) {
  calMsg.value = ''
  try {
    const { data } = await api.get('/api/calendar/token', { params: reset === true ? { reset: true } : {} })
    calUrl.value = window.location.origin + data.path
  } catch (e) { calMsg.value = e.response?.data?.detail || 'Error' }
}
function resetCal() { loadCalUrl(true).then(() => { calMsg.value = t('cal.resetOk') }) }
async function copyCal() {
  try { await navigator.clipboard.writeText(calUrl.value); calMsg.value = t('cal.copied') }
  catch { calMsg.value = calUrl.value }
}

/* ---------- 复制到剪贴板（http 下 navigator.clipboard 不可用时退回 execCommand） ---------- */
const copyMsg = ref('')
async function copyText(text, where = '') {
  let ok = false
  try { await navigator.clipboard.writeText(text); ok = true } catch {
    try {
      const ta = document.createElement('textarea')
      ta.value = text; ta.setAttribute('readonly', ''); ta.style.position = 'fixed'; ta.style.opacity = '0'
      document.body.appendChild(ta); ta.select(); ok = document.execCommand('copy'); ta.remove()
    } catch { ok = false }
  }
  const m = ok ? t('cal.copied') : text
  if (where === 'pm') { pmOk.value = ok; pmMsg.value = m } else copyMsg.value = m
  setTimeout(() => { if (where === 'pm') pmMsg.value = ''; else copyMsg.value = '' }, 2500)
}
function errText(e) {
  const d = e?.response?.data?.detail
  if (typeof d === 'string' && d) return d
  if (Array.isArray(d) && d.length) return d[0].msg || 'Error'
  return e?.response ? `HTTP ${e.response.status}` : (e?.message || 'Error')
}

/* ---------- API & MCP ---------- */
const TOKEN_PH = '<YOUR_API_TOKEN>'
const apiBase = computed(() => window.location.origin + '/api')
const mcpUrl = computed(() => window.location.origin + (sys.value?.mcp_path || '/mcp'))
const exToken = computed(() => createdToken.value || TOKEN_PH)
const mcpClaudeCmd = computed(() =>
  `claude mcp add --transport http easysub ${mcpUrl.value} --header "Authorization: Bearer ${exToken.value}"`)
const mcpJson = computed(() => JSON.stringify({
  mcpServers: { easysub: { type: 'http', url: mcpUrl.value, headers: { Authorization: `Bearer ${exToken.value}` } } }
}, null, 2))

/* ---------- 货币管理 ---------- */
const catalog = ref([])
const sysBase = ref('')
const curQ = ref('')
const curMsg = ref('')
const curOk = ref(false)
// mode: catalog（从内置目录选）| custom（自定义）| edit（编辑已有）
const curModal = reactive({ open: false, mode: 'catalog', err: '', note: '', busy: false })
const curForm = reactive({ code: '', name: '', name_zh: '', symbol: '', manual_rate: '' })
const catalogLeft = computed(() => {
  const q = curQ.value.trim().toLowerCase()
  return catalog.value.filter((c) => !currencies.value.some((x) => x.code === c.code))
    .filter((c) => !q || [c.code, c.name, c.name_zh, c.symbol].some((v) => (v || '').toLowerCase().includes(q)))
})
// 汇率并入货币列表：1 本币 ≈ x 基准货币
const rateByCode = computed(() => Object.fromEntries((rates.value.items || []).map((r) => [r.code, r.per_unit_in_base])))
function rateText(c) {
  if (c.code === rates.value.base) return t('cur.isBase')
  const v = rateByCode.value[c.code]
  if (v == null || !Number.isFinite(Number(v))) return t('cur.noRate')
  const n = Number(v)
  const s = n >= 100 ? n.toFixed(2) : n >= 1 ? n.toFixed(4) : n.toPrecision(4)
  return `1 ${c.code} ≈ ${s} ${rates.value.base}`
}
function rateOrNull(v) {
  if (v === '' || v == null) return null
  const n = Number(v)
  return Number.isFinite(n) && n > 0 ? n : null
}
function curNote(ok, m) { curOk.value = ok; curMsg.value = m }
async function loadCurrencies() {
  try { currencies.value = (await api.get('/api/currencies')).data || [] } catch { /* ignore */ }
  try { catalog.value = (await api.get('/api/currencies/catalog')).data || [] } catch { /* ignore */ }
}
async function loadSysBase() {
  try { sysBase.value = (await api.get('/api/currencies/rates')).data?.base || '' } catch { /* ignore */ }
}
async function afterCurrencyChange() { await loadCurrencies(); loadRates() }
function openCurAdd() {
  curQ.value = ''
  Object.assign(curForm, { code: '', name: '', name_zh: '', symbol: '', manual_rate: '' })
  Object.assign(curModal, { open: true, mode: 'catalog', err: '', note: '', busy: false })
}
function openCurEdit(c) {
  Object.assign(curForm, { code: c.code, name: c.name || '', name_zh: c.name_zh || '', symbol: c.symbol || '', manual_rate: c.manual_rate ?? '' })
  Object.assign(curModal, { open: true, mode: 'edit', err: '', note: '', busy: false })
}
// 从目录添加：弹窗保持打开，便于连续添加多个
async function addCatalogCurrency(code) {
  curModal.busy = true; curModal.err = ''
  try {
    await api.post('/api/currencies', { code })
    curModal.note = t('cur.added', { code })
    await afterCurrencyChange()
  } catch (e) { curModal.err = errText(e) } finally { curModal.busy = false }
}
async function submitCurrency() {
  const code = curForm.code.trim().toUpperCase()
  if (!code) return
  const body = { name: curForm.name.trim(), name_zh: curForm.name_zh.trim(), symbol: curForm.symbol.trim(), manual_rate: rateOrNull(curForm.manual_rate) }
  curModal.busy = true; curModal.err = ''
  try {
    if (curModal.mode === 'edit') {
      await api.put(`/api/currencies/${encodeURIComponent(code)}`, body)
      curNote(true, t('settings.saved'))
    } else {
      await api.post('/api/currencies', { code, ...body })
      curNote(true, t('cur.added', { code }))
    }
    curModal.open = false
    await afterCurrencyChange()
  } catch (e) { curModal.err = errText(e) } finally { curModal.busy = false }
}
async function deleteCurrency(c) {
  if (!window.confirm(t('cur.deleteConfirm', { code: c.code }))) return
  try {
    await api.delete(`/api/currencies/${encodeURIComponent(c.code)}`)
    curNote(true, t('cur.deleted', { code: c.code }))
    await afterCurrencyChange()
  } catch (e) { curNote(false, errText(e)) }
}
async function moveCurrency(i, dir) {
  const arr = [...currencies.value]
  const j = i + dir
  if (j < 0 || j >= arr.length) return
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
  currencies.value = arr
  try { await api.post('/api/currencies/reorder', { codes: arr.map((c) => c.code) }) }
  catch (e) { curNote(false, errText(e)); loadCurrencies() }
}
async function restoreCurrencies() {
  try {
    const { data } = await api.post('/api/currencies/restore-defaults')
    curNote(true, t('cur.restored', { n: data?.added ?? 0 }))
    await afterCurrencyChange()
  } catch (e) { curNote(false, errText(e)) }
}

/* ---------- 付款方式 ---------- */
const pms = ref([])
const pmMsg = ref('')
const pmOk = ref(false)
const pmForm = reactive({ show: false, id: null, name: '', icon: '', account: '', note: '', err: '' })
const emojiOpen = ref(false)
// 图标下拉：常用预设 + 当前付款方式里已经在用的 emoji（URL 图标不列入）
const EMOJI_PRESETS = ['💳', '🏦', '🏛️', '💰', '💵', '💴', '💶', '💷', '🪙', '₿', '🍎', '🅖', '🅰️', '💬', '🅿️',
  '📱', '🌐', '🛒', '🎁', '🧾', '🔐', '⭐', '❤️', '🟢', '🔵', '🟠', '🟣']
const emojiChoices = computed(() => {
  const used = pms.value.map((p) => (p.icon || '').trim()).filter((v) => v && !isImg(v))
  return [...new Set([...used, ...EMOJI_PRESETS])]
})
function pmNote(ok, m) { pmOk.value = ok; pmMsg.value = m }
async function loadMethods() { try { pms.value = (await api.get('/api/payment-methods')).data || [] } catch { /* ignore */ } }
function newMethod() {
  emojiOpen.value = false
  Object.assign(pmForm, { show: true, id: null, name: '', icon: '', account: '', note: '', err: '' })
}
function editMethod(p) {
  emojiOpen.value = false
  Object.assign(pmForm, { show: true, id: p.id, name: p.name || '', icon: p.icon || '', account: p.account || '', note: p.note || '', err: '' })
}
async function saveMethod() {
  const body = { name: pmForm.name.trim(), icon: pmForm.icon.trim(), account: pmForm.account.trim(), note: pmForm.note.trim() }
  if (!body.name) return
  // 图标只接受 emoji / 短文本，或 http(s) / 站内路径的图片地址
  if (body.icon.includes('://') && !/^https?:\/\//i.test(body.icon)) { pmForm.err = t('pm.iconBad'); return }
  pmForm.err = ''
  try {
    if (pmForm.id) await api.put(`/api/payment-methods/${pmForm.id}`, body)
    else await api.post('/api/payment-methods', body)
    pmForm.show = false; pmNote(true, t('settings.saved'))
    await loadMethods()
  } catch (e) { pmForm.err = errText(e) }
}
async function deleteMethod(p) {
  if (!window.confirm(t('pm.deleteConfirm', { name: p.name }))) return
  try { await api.delete(`/api/payment-methods/${p.id}`); pmNote(true, t('pm.deleted')); await loadMethods() }
  catch (e) { pmNote(false, errText(e)) }
}
async function moveMethod(i, dir) {
  const arr = [...pms.value]
  const j = i + dir
  if (j < 0 || j >= arr.length) return
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
  pms.value = arr
  try { await api.post('/api/payment-methods/reorder', { ids: arr.map((p) => p.id) }) }
  catch (e) { pmNote(false, errText(e)); loadMethods() }
}
async function restoreMethods() {
  try {
    const { data } = await api.post('/api/payment-methods/restore-defaults')
    pmNote(true, t('pm.restored', { n: data?.added ?? 0 })); await loadMethods()
  } catch (e) { pmNote(false, errText(e)) }
}

/* ---------- 服务库（订阅表单的服务联想 / 按分类浏览） ---------- */
const svcs = ref([])
const svcCats = ref([])
const svcQ = ref('')
const svcCat = ref('')
const svcMsg = ref('')
const svcOk = ref(false)
const svcForm = reactive({ show: false, id: null, name: '', website: '', category: 'other', icon: '', autoIcon: '', err: '', busy: false })
function svcNote(ok, m) { svcOk.value = ok; svcMsg.value = m; setTimeout(() => { if (svcMsg.value === m) svcMsg.value = '' }, 3000) }
const svcFiltered = computed(() => {
  const q = svcQ.value.trim().toLowerCase()
  return svcs.value.filter((s) => (!svcCat.value || s.category === svcCat.value)
    && (!q || s.name.toLowerCase().includes(q) || (s.domain || '').includes(q)))
})
// 按分类分组，组的顺序与分类定义一致
const svcGroups = computed(() => {
  const order = svcCats.value.map((c) => c.key)
  const groups = new Map()
  for (const s of svcFiltered.value) {
    if (!groups.has(s.category)) groups.set(s.category, { key: s.category, label: s.category_label || s.category, items: [] })
    groups.get(s.category).items.push(s)
  }
  return [...groups.values()].sort((a, b) => order.indexOf(a.key) - order.indexOf(b.key))
})
async function loadServices() {
  try { svcs.value = (await api.get('/api/services')).data || [] } catch { /* ignore */ }
  if (!svcCats.value.length) {
    try { svcCats.value = (await api.get('/api/services/categories')).data || [] } catch { /* ignore */ }
  }
}
function openSvc(s = null) {
  Object.assign(svcForm, s
    ? { show: true, id: s.id, name: s.name, website: s.website || '', category: s.category || 'other',
        icon: s.custom_icon || '', autoIcon: s.custom_icon ? '' : s.icon, err: '', busy: false }
    : { show: true, id: null, name: '', website: '', category: svcCat.value || 'other', icon: '', autoIcon: '', err: '', busy: false })
}
async function saveSvc() {
  const body = { name: svcForm.name.trim(), website: svcForm.website.trim(), category: svcForm.category, icon: svcForm.icon.trim() }
  if (!body.name) return
  if (body.icon.includes('://') && !/^https?:\/\//i.test(body.icon)) { svcForm.err = t('pm.iconBad'); return }
  svcForm.busy = true; svcForm.err = ''
  try {
    if (svcForm.id) await api.put(`/api/services/${svcForm.id}`, body)
    else await api.post('/api/services', body)
    svcForm.show = false
    svcNote(true, t('settings.saved'))
    await loadServices()
  } catch (e) { svcForm.err = errText(e) } finally { svcForm.busy = false }
}
async function deleteSvc(s) {
  if (!window.confirm(t('svc.deleteConfirm', { name: s.name }))) return
  try { await api.delete(`/api/services/${s.id}`); svcNote(true, t('svc.deleted')); await loadServices() }
  catch (e) { svcNote(false, errText(e)) }
}
async function restoreServices() {
  try {
    const { data } = await api.post('/api/services/restore-defaults')
    svcNote(true, t('svc.restored', { n: data?.added ?? 0 })); await loadServices()
  } catch (e) { svcNote(false, errText(e)) }
}

/* ---------- WebDAV ---------- */
const dav = reactive({ url: '', username: '', password: '', path: 'easysub', auto_enabled: false, keep: 14, scope: 'self' })
const davHasPwd = ref(false)
const davLast = ref(null)
const davFiles = ref([])
const davReplace = ref(false)
const davBusy = ref(false)
const davMsg = ref('')
const davOk = ref(false)
function davNote(ok, m) { davOk.value = ok; davMsg.value = m }
function humanSize(n) {
  const v = Number(n) || 0
  if (v < 1024) return `${v} B`
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(1)} KB`
  if (v < 1024 ** 3) return `${(v / 1024 / 1024).toFixed(1)} MB`
  return `${(v / 1024 ** 3).toFixed(2)} GB`
}
function applyDavConfig(c) {
  if (!c) return
  Object.assign(dav, {
    url: c.url || '', username: c.username || '', password: '', path: c.path || 'easysub',
    auto_enabled: !!c.auto_enabled, keep: c.keep || 14, scope: c.scope === 'all' ? 'all' : 'self'
  })
  davHasPwd.value = !!c.has_password
}
async function loadDav() {
  try {
    const { data } = await api.get('/api/webdav/config')
    applyDavConfig(data?.config)
    davLast.value = data?.last || null
  } catch { /* ignore */ }
}
async function loadDavFiles(show = false) {
  if (!dav.url) { davFiles.value = []; return }
  try { davFiles.value = (await api.get('/api/webdav/files')).data?.files || [] }
  catch (e) { if (show) davNote(false, errText(e)) }
}
function davBody(withAll = true) {
  const b = { url: dav.url.trim(), username: dav.username.trim(), path: dav.path.trim() || 'easysub' }
  if (dav.password) b.password = dav.password
  if (withAll) {
    b.auto_enabled = dav.auto_enabled
    b.keep = Math.max(1, Number(dav.keep) || 14)
    b.scope = auth.user?.is_admin ? dav.scope : 'self'
  }
  return b
}
async function davRun(fn) {
  davBusy.value = true; davMsg.value = ''
  try { await fn() } catch (e) { davNote(false, errText(e)) } finally { davBusy.value = false }
}
function testDav() {
  return davRun(async () => {
    const { data } = await api.post('/api/webdav/test', davBody(false))
    davNote(data?.ok !== false, data?.message || t('dav.testOk'))
  })
}
function saveDav() {
  return davRun(async () => {
    const { data } = await api.put('/api/webdav/config', davBody(true))
    applyDavConfig(data?.config)
    davNote(true, t('settings.saved'))
    loadDavFiles()
  })
}
function backupDav() {
  return davRun(async () => {
    const { data } = await api.post('/api/webdav/backup')
    davNote(true, t('dav.backupOk', { file: data?.file || '', size: humanSize(data?.size) }))
    await Promise.all([loadDav(), loadDavFiles()])
  })
}
function restoreDav(f) {
  const msg = davReplace.value ? t('dav.restoreReplaceConfirm', { name: f.name }) : t('dav.restoreConfirm', { name: f.name })
  if (!window.confirm(msg)) return
  return davRun(async () => {
    const { data } = await api.post('/api/webdav/restore', { name: f.name, replace: davReplace.value })
    davNote(true, data?.users != null
      ? t('backupAll.importOk', { users: data.users, created: data.created_users ?? 0, n: data.imported ?? 0 })
      : t('backup.importOk', { n: data?.imported ?? 0 }))
  })
}
function deleteDavFile(f) {
  if (!window.confirm(t('dav.deleteConfirm', { name: f.name }))) return
  return davRun(async () => {
    await api.delete(`/api/webdav/files/${encodeURIComponent(f.name)}`)
    davNote(true, t('dav.deleted'))
    await loadDavFiles()
  })
}

async function loadAutoBackups() {
  try { autobk.value = (await api.get('/api/backup/auto')).data } catch { /* 非管理员忽略 */ }
}
async function runAutoBackup() {
  try { await api.post('/api/backup/auto/run'); await loadAutoBackups() }
  catch (e) { calMsg.value = e.response?.data?.detail || 'Error' }
}

onMounted(async () => {
  revealTab()
  await loadCurrencies()
  try { sys.value = (await api.get('/api/system/info')).data } catch { /* ignore */ }
  loadRates()
  loadSysBase()
  loadMethods()
  loadServices()
  loadDav().then(() => loadDavFiles())
  loadTokens()
  checkUpdate()
  if (auth.user?.is_admin) loadAutoBackups()
})
</script>

<style scoped>
/* ---------- 布局：分区标签 + 自适应列数的卡片网格 ---------- */
.set-tabs { display: flex; gap: 4px; margin: -6px 0 20px; padding: 4px; overflow-x: auto; scrollbar-width: none;
  background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); width: fit-content; max-width: 100%; }
.set-tabs::-webkit-scrollbar { display: none; }
.set-tab { display: inline-flex; align-items: center; gap: 7px; flex-shrink: 0; min-height: 36px; padding: 0 14px;
  border: 0; border-radius: 8px; background: transparent; color: var(--text-soft); font: inherit; font-size: 14px;
  font-weight: 550; cursor: pointer; white-space: nowrap; transition: background-color .15s, color .15s, box-shadow .15s; }
.set-tab .hi { width: 16px; height: 16px; color: inherit; }
.set-tab:hover { color: var(--text); }
.set-tab.on { background: var(--surface); color: var(--text); box-shadow: 0 1px 2px rgba(0,0,0,.08), 0 0 0 1px var(--border); }
.set-tab:focus-visible { outline: none; box-shadow: var(--focus); }

.set-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 440px), 1fr)); gap: 16px; align-items: start; }
.set-grid > .card { margin: 0; min-width: 0; }
.span-all { grid-column: 1 / -1; min-width: 0; }
.head-acts { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.head-acts .btn span { display: inline-flex; }
.mcp-grid, .dav-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 380px), 1fr)); gap: 0 28px; align-items: start; }
.dav-files .sh-row { margin-top: 12px; }

/* ---------- 卡片标题 ---------- */
.sh { display: flex; align-items: center; gap: 9px; margin: 0 0 6px; }
.sh + :not(.desc) { margin-top: 10px; }
.hi { display: inline-flex; width: 18px; height: 18px; flex-shrink: 0; color: var(--text-soft); }
.sh-row { display: flex; justify-content: space-between; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 6px; }
.sh-row .sh { margin: 0; }
.sub-h { display: flex; align-items: center; gap: 8px; font-size: 14px; margin: 0 0 4px; }
.sub-h .hi { width: 16px; height: 16px; }
.desc { font-size: 13px; margin: 0 0 14px; line-height: 1.6; }
.hint { font-size: 12.5px; margin: 8px 0 0; line-height: 1.6; }
.hint.top { margin: 0 0 8px; }
hr { border: none; border-top: 1px solid var(--border); margin: 18px 0; }
.sect > label:first-of-type { margin-top: 8px; }

/* ---------- 字段排布 ---------- */
.pair { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 12px; }
.trio { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 0 12px; }
.actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-top: 14px; }
.actions .btn span, .ver-row .btn span { display: inline-flex; }
.file-btn { width: auto; margin: 0; color: var(--text); font-size: 14px; font-weight: 550; }
.auto-w { width: auto; min-width: 110px; }
.toggle-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-top: 12px; }

/* ---------- 状态提示 ---------- */
.msg { display: flex; align-items: flex-start; gap: 6px; margin: 12px 0 0; font-size: 13px; line-height: 1.5;
  overflow-wrap: anywhere; }
.msg.inline { margin: 0; align-items: center; }
.msg.ok { color: var(--ok); }
.msg.err { color: var(--late); }
.msg.neutral { color: var(--text-soft); }
.mi { display: inline-flex; width: 15px; height: 15px; flex-shrink: 0; margin-top: 2px; }
.msg.inline .mi { margin-top: 0; }

/* ---------- iOS 风格开关（label.switch > checkbox + 文本） ---------- */
.switch { display: inline-flex; align-items: center; gap: 10px; margin: 0; width: auto; min-height: 40px;
  font-size: 14px; font-weight: 500; color: var(--text); cursor: pointer; user-select: none; }
.switch input { appearance: none; -webkit-appearance: none; position: relative; flex-shrink: 0;
  width: 36px; height: 22px; min-height: 0; margin: 0; padding: 0; border: 0; border-radius: 999px;
  background: var(--border-strong); cursor: pointer; transition: background-color .2s var(--ease-out); }
.switch input::before { content: ''; position: absolute; top: 2px; left: 2px; width: 18px; height: 18px;
  border-radius: 50%; background: #fff; box-shadow: 0 1px 3px rgba(0,0,0,.25);
  transition: transform .22s var(--ease-out); }
.switch input:checked { background: var(--primary); }
.switch input:checked::before { transform: translateX(14px); background: var(--on-primary); }
.switch input:focus-visible { outline: none; box-shadow: var(--focus); }

/* ---------- 主题色板 ---------- */
.theme-picker { display: grid; grid-template-columns: repeat(auto-fill, minmax(64px, 1fr)); gap: 8px; }
.th { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 8px 4px 6px; min-height: 40px;
  border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface); cursor: pointer;
  font: inherit; color: var(--text-soft); transition: border-color .15s, background-color .15s, color .15s; }
.th:hover { border-color: var(--border-strong); color: var(--text); }
.th.on { border-color: var(--primary); color: var(--text); box-shadow: inset 0 0 0 1px var(--primary); }
.sw { position: relative; width: 34px; height: 24px; border-radius: var(--radius-xs);
  box-shadow: inset 0 0 0 1px rgba(0,0,0,.12); overflow: hidden; }
.sw i { position: absolute; right: 4px; bottom: 4px; width: 10px; height: 10px; border-radius: 50%; }
.th-n { font-size: 12px; font-weight: 550; line-height: 1.2; }

/* ---------- 安全 ---------- */
.sec-row { display: flex; justify-content: space-between; align-items: center; gap: 10px 12px; flex-wrap: wrap; }
.sec-l { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.inline-form { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-top: 0; }
.twofa-r .inline-form { margin-top: 10px; }
.otp { width: 130px; font-family: var(--mono); letter-spacing: .1em; }
.tk-name { width: 150px; }
.twofa-box { display: flex; gap: 16px; align-items: flex-start; margin: 14px 0 0; flex-wrap: wrap;
  padding: 14px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2); }
.twofa-box .qr { width: 140px; height: 140px; border-radius: var(--radius-xs); background: #fff;
  border: 1px solid var(--border); }
.twofa-r { flex: 1; min-width: 180px; }
.secret, .code { display: block; font-family: var(--mono); font-size: 12.5px; line-height: 1.55;
  padding: 8px 10px; border-radius: var(--radius-xs); background: var(--surface-2); border: 1px solid var(--border);
  color: var(--text); overflow-wrap: anywhere; word-break: break-all; }
.twofa-box .secret { background: var(--surface); }
.token-once { margin-top: 4px; }
.token-once .code { margin-top: 8px; }

/* ---------- 列表（Token / 自动备份） ---------- */
.item-list { list-style: none; padding: 0; margin: 12px 0 0; border: 1px solid var(--border);
  border-radius: var(--radius-sm); overflow: hidden; }
.item-list li { display: flex; align-items: center; justify-content: space-between; gap: 10px;
  padding: 9px 8px 9px 12px; font-size: 13px; }
.item-list li + li { border-top: 1px solid var(--border); }
.il-main { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.il-main b { font-weight: 550; overflow-wrap: anywhere; }
.il-main .muted { font-size: 12px; overflow-wrap: anywhere; }
.item-list .btn span { display: inline-flex; }

/* ---------- 列表操作 / 复制 ---------- */
.il-acts { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }
.il-acts .btn span { display: inline-flex; }
.rot-up { transform: rotate(-90deg); }
.rot-down { transform: rotate(90deg); }
.rot-left { transform: rotate(180deg); }
.rot-up, .rot-down, .rot-left { display: inline-flex; }
.mono-s { font-family: var(--mono); font-size: 12.5px; }
.copy-row { display: flex; align-items: flex-start; gap: 8px; }
.copy-row .code { flex: 1; min-width: 0; margin: 0; }
.copy-row .btn span { display: inline-flex; }
.pre { white-space: pre-wrap; word-break: break-all; }

/* ---------- 货币与汇率 ---------- */
.cur-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 340px), 1fr)); gap: 8px; }
.cur-item { display: flex; align-items: center; gap: 12px; min-width: 0; padding: 10px 8px 10px 10px;
  border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface);
  transition: border-color .15s; }
.cur-item:hover { border-color: var(--border-strong); }
.cur-badge { display: flex; flex-direction: column; align-items: center; justify-content: center; flex-shrink: 0;
  width: 54px; height: 44px; border-radius: 9px; background: var(--surface-2); line-height: 1.15; }
.cur-badge .mono-s { font-weight: 650; font-size: 12.5px; color: var(--text); }
.cur-sym { color: var(--text-soft); font-size: 12px; }
.cur-main { flex: 1; min-width: 0; }
.cur-name { display: flex; align-items: center; gap: 6px; min-width: 0; }
.cur-name b { font-weight: 600; font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cur-name .tag { flex-shrink: 0; padding: 0 7px; font-size: 11px; }
.cur-sub { font-size: 12px; margin-top: 1px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cur-rate { font-size: 12.5px; color: var(--text); margin-top: 1px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.tag.manual { padding: 0 6px; font-size: 10.5px; margin-left: 6px; vertical-align: 1px; }
.cur-item .il-acts { gap: 2px; }
/* 排序按钮悬停时才显示，给名称和汇率留出宽度 */
.cur-move { display: inline-flex; flex-direction: column; gap: 2px; opacity: 0; transition: opacity .15s; }
.cur-move .btn.xs { width: 24px; height: 18px; min-height: 18px; padding: 2px; }
.cur-item:hover .cur-move, .cur-item:focus-within .cur-move { opacity: 1; }
@media (hover: none) { .cur-move { opacity: 1; } }

/* ---------- 付款方式 ---------- */
.pm-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr)); gap: 10px; }
.pm-item { position: relative; display: flex; align-items: flex-start; gap: 12px; min-width: 0; padding: 12px 10px 12px 12px;
  border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface); transition: border-color .15s; }
.pm-item:hover { border-color: var(--border-strong); }
.pm-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 3px; padding-top: 1px; }
.pm-name { font-weight: 600; font-size: 14px; overflow-wrap: anywhere; }
.pm-acc { display: inline-flex; align-items: center; gap: 4px; min-width: 0; }
.pm-acc code { color: var(--text); overflow-wrap: anywhere; }
.pm-note { font-size: 12px; white-space: pre-line; overflow-wrap: anywhere; }
.il-acts.col { flex-direction: column; gap: 4px; }
/* 排序按钮：悬停卡片时出现在左下角，不占布局 */
.pm-move { position: absolute; left: 8px; bottom: 6px; display: flex; gap: 2px; opacity: 0; transition: opacity .15s; }
.pm-item:hover .pm-move, .pm-item:focus-within .pm-move { opacity: 1; }
.pm-move .btn { background: var(--surface); }
.btn.sm.icon.xs { width: 26px; height: 26px; min-height: 26px; padding: 5px; }

/* ---------- 服务库 ---------- */
.svc-tools { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 4px; }
.svc-tools input { flex: 1; min-width: 180px; max-width: 360px; }
.svc-count { font-size: 13px; margin-left: auto; }
.svc-group { margin-top: 18px; }
.svc-group-t { display: flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 600; color: var(--text-soft); margin-bottom: 8px; }
.svc-group-t .tag { padding: 0 7px; font-size: 11px; }
.svc-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 240px), 1fr)); gap: 8px; }
.svc-item { display: flex; align-items: center; gap: 10px; min-width: 0; padding: 8px 6px 8px 10px;
  border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface); transition: border-color .15s; }
.svc-item:hover { border-color: var(--border-strong); }
.svc-main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.svc-main b { font-weight: 600; font-size: 13.5px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.svc-main .muted { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
/* 操作按钮悬停时才显示（触屏设备始终显示） */
.svc-item .il-acts { gap: 2px; opacity: 0; transition: opacity .15s; }
.svc-item:hover .il-acts, .svc-item:focus-within .il-acts { opacity: 1; }
@media (hover: none) { .svc-item .il-acts, .pm-move { opacity: 1; } }

/* ---------- 弹窗内部 ---------- */
.modal-seg { display: flex; margin-bottom: 4px; }
.modal-seg button { flex: 1; }
.modal-search { margin-top: 12px; }
.pick-list { max-height: 46vh; overflow: auto; }
.pick-list .il-main b { font-size: 13.5px; }
.modal-acts { display: flex; justify-content: flex-end; gap: 10px; margin-top: 20px; }
.icon-field { display: flex; align-items: center; gap: 10px; }
.icon-field input { flex: 1; min-width: 0; }
.icon-field .btn { flex-shrink: 0; }
.icon-field .btn span { display: inline-flex; }
.chev { width: 14px; height: 14px; transform: rotate(90deg); transition: transform .15s; }
.chev.open { transform: rotate(-90deg); }
.emoji-pop { display: grid; grid-template-columns: repeat(auto-fill, minmax(40px, 1fr)); gap: 4px; margin-top: 8px;
  padding: 8px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2);
  max-height: 180px; overflow: auto; }
.emoji-i { height: 40px; border: 1px solid transparent; border-radius: 8px; background: var(--surface); font-size: 20px;
  cursor: pointer; transition: border-color .12s, transform .12s; }
.emoji-i:hover { border-color: var(--border-strong); transform: scale(1.06); }
.emoji-i.on { border-color: var(--primary); box-shadow: inset 0 0 0 1px var(--primary); }
.emoji-i:focus-visible { outline: none; box-shadow: var(--focus); }

/* ---------- WebDAV ---------- */
.last-ok { color: var(--ok); }
.last-err { color: var(--late); overflow-wrap: anywhere; }

.rate-msg { color: var(--text); }

/* ---------- 日历 ---------- */
.cal-url { margin-top: 12px; font-family: var(--mono); font-size: 12.5px; background: var(--surface-2); }

/* ---------- 版本 ---------- */
.ver-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; margin-top: 10px; }
.ver { font-size: 14px; color: var(--text-soft); }
.ver b { color: var(--text); margin-left: 4px; }
.upd-box { margin-top: 12px; padding: 12px 14px; background: var(--surface-2); border-radius: var(--radius-sm);
  border: 1px solid var(--border); }
.upd-how { font-size: 13px; margin: 0 0 8px; }
.upd-box .code { background: var(--surface); }

/* ---------- 系统信息：定义列表 ---------- */
.kv { margin: 6px 0 0; }
.kv > div { display: flex; justify-content: space-between; align-items: baseline; gap: 16px; padding: 9px 0;
  border-bottom: 1px solid var(--border); font-size: 14px; }
.kv > div:last-child { border-bottom: 0; padding-bottom: 0; }
.kv dt { color: var(--text-soft); flex-shrink: 0; }
.kv dd { margin: 0; text-align: right; font-weight: 550; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; min-width: 0; }
.db-ok { display: inline-flex; align-items: center; gap: 7px; }

.tabular { font-variant-numeric: tabular-nums; }

@media (max-width: 720px) {
  .set-tabs { width: auto; margin: -4px -16px 16px; border-radius: 0; border-left: 0; border-right: 0; padding: 4px 12px; }
  .set-tab { padding: 0 12px; }
  .set-grid { gap: 12px; }
  .head-acts { width: 100%; }
  .head-acts .btn { flex: 1 1 auto; }
  .svc-tools input { max-width: none; }
  .svc-count { margin-left: 0; }
  .icon-field { flex-wrap: wrap; }
  .modal-acts .btn { flex: 1; }
  .pair { grid-template-columns: minmax(0, 1fr); }
  .btn.sm { min-height: 40px; }
  .btn.sm.icon { width: 40px; }
  .actions .btn, .actions .file-btn { flex: 1 1 auto; }
  .tk-name { flex: 1; width: auto; min-width: 0; }
  .sec-row > .inline-form { width: 100%; flex-wrap: nowrap; }
  .twofa-box { flex-direction: column; align-items: center; }
  .twofa-r { width: 100%; min-width: 0; }
  .otp { flex: 1; width: auto; min-width: 0; }
}
</style>
