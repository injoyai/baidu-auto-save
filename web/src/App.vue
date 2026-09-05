<template>
  <router-view v-slot="{ Component }">
    <transition name="page" mode="out-in">
      <component :is="Component" />
    </transition>
  </router-view>
</template>

<style>
/* ===== 设计 token：极光 Aurora =====
   深空 #0B1424 · 极光渐变 青→紫→天蓝 · 云白内容区 */
:root {
  --space: #0b1424;
  --space-soft: #16233c;
  --ink: #122036;
  --ink-soft: #5b6b83;
  --aurora-teal: #14b8a6;
  --aurora-violet: #8b5cf6;
  --aurora-sky: #38bdf8;
  --grad: linear-gradient(120deg, #14b8a6 0%, #38bdf8 50%, #8b5cf6 100%);
  --cloud: #f3f6fa;
  --line: #e4eaf2;
  --mono: 'JetBrains Mono', 'Cascadia Code', Consolas, 'Courier New', monospace;
}

body {
  margin: 0;
  font-family: -apple-system, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB',
    'Microsoft YaHei', sans-serif;
  background: var(--cloud);
  color: var(--ink);
  -webkit-font-smoothing: antialiased;
}

/* Element Plus 主题对齐：主色取极光青 */
:root {
  --el-color-primary: var(--aurora-teal);
  --el-color-primary-dark-2: #0d9488;
  --el-color-primary-light-3: #3fcfc0;
  --el-color-primary-light-5: #79e0d5;
  --el-color-primary-light-7: #aeeae4;
  --el-color-primary-light-8: #ccf2ee;
  --el-color-primary-light-9: #e7f9f7;
  --el-border-radius-base: 10px;
  --el-border-radius-small: 7px;
  --el-text-color-primary: var(--ink);
  --el-text-color-regular: var(--ink-soft);
  --el-border-color: var(--line);
  --el-border-color-light: var(--line);
  --el-fill-color-light: #edf2f8;
}

/* 路由切换：滑动淡入淡出 */
.page-enter-active,
.page-leave-active {
  transition: opacity 0.22s ease, transform 0.22s ease;
}
.page-enter-from {
  opacity: 0;
  transform: translateY(10px);
}
.page-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

/* 数据等宽 */
.mono {
  font-family: var(--mono);
  font-size: 0.92em;
  letter-spacing: -0.01em;
}

/* 极光渐变文字 */
.grad-text {
  background: var(--grad);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

/* 页面骨架 */
.page-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 10px;
}
.page-title {
  font-size: 21px;
  font-weight: 800;
  letter-spacing: 0.02em;
  margin: 0;
}
.page-title::before {
  content: '';
  display: inline-block;
  width: 9px;
  height: 9px;
  border-radius: 3px;
  background: var(--grad);
  margin-right: 10px;
  transform: rotate(45deg);
  vertical-align: 2px;
}

/* 卡片：柔和悬浮 + 微升起 */
.el-card {
  border: 1px solid var(--line);
  box-shadow: 0 1px 3px rgba(18, 32, 54, 0.05) !important;
  transition: box-shadow 0.25s ease, transform 0.25s ease;
}
.el-card:hover {
  box-shadow: 0 10px 28px rgba(18, 32, 54, 0.1) !important;
  transform: translateY(-2px);
}
.el-card__header {
  font-weight: 700;
  border-bottom: 1px solid var(--line);
}

/* 表格 */
.el-table {
  --el-table-header-bg-color: #edf2f8;
  --el-table-header-text-color: var(--ink-soft);
  --el-table-row-hover-bg-color: #e7f9f7;
}
.el-table th.el-table__cell {
  font-weight: 700;
  font-size: 12.5px;
}

/* 主按钮：极光渐变底 + 光泽扫过（plain 变体不适用，保持浅底深字） */
.el-button--primary:not(.is-plain):not(.is-text):not(.is-link) {
  background: var(--grad) !important;
  border: none !important;
  position: relative;
  overflow: hidden;
  transition: box-shadow 0.25s ease, transform 0.15s ease;
}
.el-button--primary:not(.is-plain):not(.is-text):not(.is-link)::after {
  content: '';
  position: absolute;
  top: 0;
  left: -80%;
  width: 50%;
  height: 100%;
  background: linear-gradient(105deg, transparent, rgba(255, 255, 255, 0.35), transparent);
  transform: skewX(-20deg);
  transition: left 0.55s ease;
}
.el-button--primary:hover::after {
  left: 130%;
}
.el-button--primary:not(.is-plain):not(.is-text):not(.is-link):hover {
  box-shadow: 0 6px 18px rgba(20, 184, 166, 0.4);
  transform: translateY(-1px);
}
.el-button--primary:not(.is-plain):not(.is-text):not(.is-link):active {
  transform: translateY(0) scale(0.98);
}

/* 进度条：极光渐变 + 流光 */
.el-progress-bar__outer {
  background: #e9eef5 !important;
}
.el-progress-bar__inner {
  background: var(--grad) !important;
  position: relative;
  overflow: hidden;
}
.el-progress-bar__inner::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(105deg, transparent 30%, rgba(255, 255, 255, 0.45) 50%, transparent 70%);
  animation: bar-shine 2.4s ease-in-out infinite;
}
@keyframes bar-shine {
  0% { transform: translateX(-100%); }
  60%, 100% { transform: translateX(100%); }
}

/* 侧栏菜单激活 */
.el-menu-item.is-active {
  background: linear-gradient(90deg, rgba(20, 184, 166, 0.22), rgba(139, 92, 246, 0.12)) !important;
  border-right: 3px solid transparent;
  border-image: var(--grad) 1;
}

/* 输入框聚焦极光描边 */
.el-input__wrapper.is-focus,
.el-textarea__inner:focus {
  box-shadow: 0 0 0 1.5px var(--aurora-teal), 0 0 0 4px rgba(20, 184, 166, 0.14) !important;
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
</style>
