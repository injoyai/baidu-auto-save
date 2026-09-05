<script setup>
// 入场动画：IntersectionObserver 进入视口时淡入上移（瀑布延迟用 style 传）
</script>

<template>
  <div class="reveal">
    <slot />
  </div>
</template>

<style scoped>
.reveal {
  opacity: 0;
  transform: translateY(16px);
  animation: reveal-in 0.55s cubic-bezier(0.22, 1, 0.36, 1) forwards;
  animation-delay: var(--reveal-delay, 0s);
}
@keyframes reveal-in {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
