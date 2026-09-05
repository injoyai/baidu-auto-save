<script setup>
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()

function logout() {
  auth.logout()
  router.push('/login')
}
</script>

<template>
  <el-container class="layout">
    <el-aside width="224px" class="aside">
      <!-- 品牌区：极光文件流动 -->
      <div class="brand">
        <div class="flow" aria-hidden="true"><span></span><span></span><span></span></div>
        <div class="brand-name">网盘<span class="grad-text">自动转存</span></div>
        <div class="brand-sub mono">baidu-auto-save</div>
      </div>

      <el-menu
        router
        :default-active="$route.path"
        background-color="transparent"
        text-color="#9db0cb"
        active-text-color="#ffffff"
        class="menu"
      >
        <el-menu-item index="/">仪表盘</el-menu-item>
        <el-menu-item index="/accounts">账号管理</el-menu-item>
        <el-menu-item index="/tasks">任务管理</el-menu-item>
        <el-menu-item index="/logs">转存日志</el-menu-item>
        <el-menu-item index="/settings">设置</el-menu-item>
      </el-menu>

      <div class="aside-foot mono">v1.0</div>
    </el-aside>

    <el-container>
      <el-header class="topbar">
        <div class="topbar-title">{{ $route.meta?.title || '' }}</div>
        <el-button text @click="logout">退出登录</el-button>
      </el-header>
      <el-main class="main">
        <router-view v-slot="{ Component }">
          <transition name="fade-slide" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout {
  min-height: 100vh;
}
.aside {
  position: relative;
  background: linear-gradient(180deg, var(--space) 0%, #081020 100%);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
/* 侧栏极光呼吸（低调） */
.aside::before {
  content: '';
  position: absolute;
  width: 320px; height: 320px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(20, 184, 166, 0.16) 0%, transparent 65%);
  top: -80px; left: -100px;
  animation: breathe 9s ease-in-out infinite alternate;
  pointer-events: none;
}
.aside::after {
  content: '';
  position: absolute;
  width: 260px; height: 260px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(139, 92, 246, 0.12) 0%, transparent 65%);
  bottom: -60px; right: -80px;
  animation: breathe 12s ease-in-out infinite alternate-reverse;
  pointer-events: none;
}
@keyframes breathe {
  from { opacity: 0.5; transform: scale(1); }
  to { opacity: 1; transform: scale(1.15); }
}

.brand {
  padding: 26px 22px 22px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.07);
  position: relative;
  z-index: 1;
}
.brand-name {
  color: #fff;
  font-size: 17px;
  font-weight: 800;
  letter-spacing: 0.06em;
  margin-top: 14px;
}
.brand-sub {
  color: #55688a;
  font-size: 11px;
  margin-top: 4px;
}
.flow {
  display: flex;
  gap: 7px;
}
.flow span {
  width: 13px; height: 13px;
  border-radius: 4px;
  animation: drift 2.2s ease-in-out infinite;
}
.flow span:nth-child(1) { background: #14b8a6; }
.flow span:nth-child(2) { background: #38bdf8; animation-delay: 0.25s; opacity: 0.8; }
.flow span:nth-child(3) { background: #8b5cf6; animation-delay: 0.5s; opacity: 0.6; }
@keyframes drift {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-7px); }
}

.menu {
  border-right: none;
  padding-top: 12px;
  flex: 1;
  position: relative;
  z-index: 1;
}
.menu :deep(.el-menu-item) {
  height: 46px;
  margin: 3px 12px;
  border-radius: 8px;
  transition: transform 0.2s ease, background 0.2s ease, padding-left 0.2s ease;
}
.menu :deep(.el-menu-item:hover) {
  transform: translateX(4px);
  background: rgba(255, 255, 255, 0.06);
}
.aside-foot {
  color: #55688a;
  font-size: 11px;
  padding: 16px 22px;
  position: relative;
  z-index: 1;
}

/* 顶栏：玻璃拟态 */
.topbar {
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-bottom: 1px solid var(--line);
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 58px;
  position: sticky;
  top: 0;
  z-index: 10;
}
.topbar-title {
  font-weight: 700;
  font-size: 14px;
  color: var(--ink-soft);
}
.main {
  padding: 26px;
}

/* 内容区切换动画 */
.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.fade-slide-enter-from {
  opacity: 0;
  transform: translateX(14px);
}
.fade-slide-leave-to {
  opacity: 0;
  transform: translateX(-10px);
}
</style>
