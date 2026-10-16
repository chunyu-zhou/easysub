<!--
  渲染错误兜底：Vue 里子组件渲染抛异常会导致整棵子树被卸载，页面上表现为「某一栏整块消失」，
  用户完全看不出发生了什么（debug13 的 Webhook、debug16 的企业微信自建应用都是这个现象）。
  包一层边界后，出错的只是这一个面板，且会把错误信息显示出来便于反馈，页面其余部分照常可用。
-->
<template>
  <div v-if="err" class="sect eb" role="alert">
    <div class="eb-head">
      <span class="eb-i" v-html="icon('alert')"></span>
      <h3>{{ t('eb.title') }}</h3>
    </div>
    <p class="eb-tip">{{ t('eb.tip') }}</p>
    <pre>{{ err }}</pre>
    <p class="eb-contact">TG:@Aiden_SU · E-mail:aidensu8182@gmail.com</p>
  </div>
  <slot v-else />
</template>

<script setup>
import { onErrorCaptured, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { icon } from '../icons'

const { t } = useI18n()
const err = ref('')

onErrorCaptured((e) => {
  err.value = String(e?.message || e)
  console.error('[EasySub] 面板渲染失败：', e)
  return false // 不再向上冒泡，保住页面其余部分
})
</script>

<style scoped>
.eb { background: var(--late-soft); border: 1px solid color-mix(in srgb, var(--late) 30%, var(--border));
  border-radius: var(--radius); padding: 18px 20px; min-width: 0; }
.eb-head { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.eb-i { display: inline-flex; width: 20px; height: 20px; color: var(--late); flex-shrink: 0; }
.eb-i :deep(svg) { width: 20px; height: 20px; }
.eb h3 { margin: 0; font-size: 16px; color: var(--text); }
.eb-tip { margin: 0 0 12px; font-size: 13.5px; color: var(--text-soft); line-height: 1.65; }
.eb pre { margin: 0; padding: 10px 12px; border-radius: var(--radius-sm); background: var(--surface);
  border: 1px solid var(--border); font-family: var(--mono); font-size: 12px; line-height: 1.55;
  white-space: pre-wrap; overflow-wrap: anywhere; color: var(--late); max-height: 240px; overflow-y: auto; }
.eb-contact { margin: 12px 0 0; font-size: 12.5px; color: var(--text-soft); overflow-wrap: anywhere; }
@media (max-width: 720px) {
  .eb { padding: 16px; }
}
</style>
