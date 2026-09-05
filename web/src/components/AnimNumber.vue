<script setup>
// 数字滚动：值变化时缓动计数（ease-out cubic）
import { ref, watch } from 'vue'

const props = defineProps({ value: { type: Number, default: 0 } })
const display = ref(0)

watch(
  () => props.value,
  (to) => {
    const from = display.value
    const start = performance.now()
    const dur = 900
    const ease = (t) => 1 - Math.pow(1 - t, 3)
    const tick = (now) => {
      const p = Math.min(1, (now - start) / dur)
      display.value = Math.round(from + (to - from) * ease(p))
      if (p < 1) requestAnimationFrame(tick)
    }
    requestAnimationFrame(tick)
  },
  { immediate: true }
)
</script>

<template><span>{{ display }}</span></template>
