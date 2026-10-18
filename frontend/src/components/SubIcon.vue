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
/* 无边框图标：图片按比例缩放进可视区域（contain，不裁切），并限制最大尺寸，保证图标完整显示 */
.sub-icon { --box: min(calc(var(--sz) * .84), 48px);
  position: relative; width: var(--sz); height: var(--sz); flex-shrink: 0; display: inline-flex;
  align-items: center; justify-content: center; }
img { position: absolute; top: 50%; left: 50%; width: var(--box); height: var(--box); transform: translate(-50%, -50%);
  object-fit: contain; border-radius: 22%; opacity: 0; transition: opacity .2s ease; }
img.ready { opacity: 1; }
.glyph { font-size: min(calc(var(--sz) * .62), 40px); line-height: 1; }
/* 首字占位：没有图片时用浅色圆底，避免字母悬空 */
.initial { display: inline-flex; align-items: center; justify-content: center; width: var(--box); height: var(--box);
  border-radius: 50%; background: var(--surface-2); font-size: calc(var(--sz) * .4); font-weight: 650; color: var(--text-soft); }
</style>
