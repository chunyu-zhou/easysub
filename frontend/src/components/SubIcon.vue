<template>
  <span ref="root" class="sub-icon" :style="{ '--sz': size + 'px' }" aria-hidden="true">
    <!-- 占位（首字 / emoji）立即显示；图片 load 成功后才替换，失败或超时则一直保留占位 -->
    <span v-if="!loaded" :class="glyph ? 'glyph' : 'initial'">{{ glyph || initial }}</span>
    <img v-if="src && !failed" :src="src" alt="" loading="lazy" decoding="async" referrerpolicy="no-referrer"
         :class="{ ready: loaded }" @load="onLoad" @error="onFail" />
  </span>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { isImg } from '../lifeline'

// 订阅图标：上传/图库图片、用户填的 emoji，或取名称首字作为占位。
// 外部图标不可达时绝不无限转圈：4 秒内没加载完就放弃，保留占位。
const TIMEOUT_MS = 4000
const props = defineProps({ icon: String, name: String, size: { type: Number, default: 40 } })

const root = ref(null)
const loaded = ref(false)
const failed = ref(false)
const src = computed(() => (isImg(props.icon) ? props.icon : ''))
const glyph = computed(() => (!isImg(props.icon) && props.icon ? props.icon : ''))
const initial = computed(() => (props.name || '?').trim().charAt(0).toUpperCase() || '?')

let timer = null
let observer = null
function clearTimer() { if (timer) { clearTimeout(timer); timer = null } }
function startTimer() {
  clearTimer()
  if (!src.value || loaded.value || failed.value) return
  timer = setTimeout(() => { if (!loaded.value) failed.value = true }, TIMEOUT_MS)
}
// loading="lazy" 的图片进入视口才开始请求，所以超时从可见时开始计
function arm() {
  clearTimer()
  if (observer) { observer.disconnect(); observer = null }
  if (!src.value) return
  if (typeof IntersectionObserver === 'undefined' || !root.value) return startTimer()
  observer = new IntersectionObserver((entries) => {
    if (entries.some((e) => e.isIntersecting)) {
      observer.disconnect(); observer = null
      startTimer()
    }
  }, { rootMargin: '200px' })
  observer.observe(root.value)
}
function onLoad() { if (!failed.value) { loaded.value = true; clearTimer() } }
function onFail() { failed.value = true; loaded.value = false; clearTimer() }

watch(() => props.icon, () => { loaded.value = false; failed.value = false; arm() })
onMounted(arm)
onBeforeUnmount(() => { clearTimer(); if (observer) observer.disconnect() })
</script>

<style scoped>
/* 圆形图标：图片铺满并按圆形裁切，favicon 自带的方形底色/四角不会露出来 */
.sub-icon { position: relative; width: var(--sz); height: var(--sz); flex-shrink: 0; display: inline-flex; align-items: center;
  justify-content: center; border-radius: 50%; background: var(--surface-2); overflow: hidden; isolation: isolate; }
/* 细描边画在图片上层，浅色图标在浅色背景上也有边界 */
.sub-icon::after { content: ''; position: absolute; inset: 0; border-radius: inherit; pointer-events: none;
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--text) 10%, transparent); }
img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; border-radius: inherit;
  opacity: 0; transition: opacity .2s ease; }
img.ready { opacity: 1; }
.glyph { font-size: calc(var(--sz) * .55); line-height: 1; }
.initial { font-size: calc(var(--sz) * .42); font-weight: 650; color: var(--text-soft); }
</style>
