<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import axios from 'axios'
import { useAuthStore } from '../stores/auth'

const pwd = ref('')
const loading = ref(false)
const router = useRouter()
const auth = useAuthStore()

// 卡片 3D 倾斜跟随鼠标
const tilt = ref({ rx: 0, ry: 0 })
function onMove(e) {
  const r = e.currentTarget.getBoundingClientRect()
  const x = (e.clientX - r.left) / r.width - 0.5
  const y = (e.clientY - r.top) / r.height - 0.5
  tilt.value = { rx: -y * 10, ry: x * 12 }
}
function onLeave() {
  tilt.value = { rx: 0, ry: 0 }
}

async function login() {
  if (!pwd.value) return ElMessage.warning('请输入密码')
  loading.value = true
  try {
    const res = await axios.post('/api/v1/login', { password: pwd.value })
    if (res.data.code !== 0) throw new Error(res.data.message)
    auth.setToken(res.data.data.token)
    router.push('/')
  } catch (e) {
    ElMessage.error(e.response?.data?.message || e.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <!-- 极光幕布：三团色块缓慢漂移 -->
    <div class="aurora" aria-hidden="true">
      <span class="a1"></span><span class="a2"></span><span class="a3"></span>
    </div>
    <!-- 星尘 -->
    <div class="stars" aria-hidden="true">
      <i v-for="i in 26" :key="i" :style="{ '--d': i * 0.7 + 's', '--x': (i * 137) % 100 + '%', '--y': (i * 61) % 100 + '%' }"></i>
    </div>

    <div
      class="login-card"
      :style="{ transform: `perspective(900px) rotateX(${tilt.rx}deg) rotateY(${tilt.ry}deg)` }"
      @mousemove="onMove"
      @mouseleave="onLeave"
    >
      <div class="flow" aria-hidden="true"><span></span><span></span><span></span></div>
      <h1 class="title">网盘<span class="grad-text">自动转存</span></h1>
      <p class="sub mono">baidu-auto-save</p>

      <el-form @submit.prevent="login">
        <el-form-item>
          <el-input
            v-model="pwd"
            type="password"
            placeholder="登录密码"
            size="large"
            show-password
            @keyup.enter="login"
          />
        </el-form-item>
        <el-button type="primary" size="large" class="login-btn" :loading="loading" @click="login">
          进 入
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<style scoped>
.login-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--space);
  position: relative;
  overflow: hidden;
}

/* 极光 */
.aurora span {
  position: absolute;
  border-radius: 50%;
  filter: blur(90px);
  opacity: 0.5;
  mix-blend-mode: screen;
}
.aurora .a1 {
  width: 560px; height: 560px;
  background: radial-gradient(circle, #14b8a6 0%, transparent 65%);
  top: -12%; left: -8%;
  animation: aurora-drift 16s ease-in-out infinite alternate;
}
.aurora .a2 {
  width: 480px; height: 480px;
  background: radial-gradient(circle, #8b5cf6 0%, transparent 65%);
  bottom: -16%; right: -6%;
  animation: aurora-drift 20s ease-in-out infinite alternate-reverse;
}
.aurora .a3 {
  width: 380px; height: 380px;
  background: radial-gradient(circle, #38bdf8 0%, transparent 65%);
  top: 32%; left: 55%;
  animation: aurora-drift 24s ease-in-out infinite alternate;
}
@keyframes aurora-drift {
  0% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(60px, -40px) scale(1.12); }
  100% { transform: translate(-50px, 36px) scale(0.94); }
}

/* 星尘闪烁 */
.stars i {
  position: absolute;
  width: 2px; height: 2px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.7);
  left: var(--x); top: var(--y);
  animation: twinkle 3.4s ease-in-out infinite;
  animation-delay: var(--d);
}
@keyframes twinkle {
  0%, 100% { opacity: 0.15; }
  50% { opacity: 0.85; }
}

/* 卡片：入场 + 3D 跟随 */
.login-card {
  width: 372px;
  background: rgba(255, 255, 255, 0.96);
  border-radius: 18px;
  padding: 42px 38px 38px;
  box-shadow: 0 24px 70px rgba(2, 8, 20, 0.55), 0 0 0 1px rgba(255, 255, 255, 0.06);
  position: relative;
  z-index: 1;
  transition: transform 0.18s ease-out;
  animation: card-in 0.7s cubic-bezier(0.22, 1, 0.36, 1);
}
@keyframes card-in {
  from { opacity: 0; transform: translateY(28px) scale(0.97); }
  to { opacity: 1; }
}
/* 鼠标移动时立即响应（覆盖入场动画结束态） */
.login-card[style] {
  animation: card-in 0.7s cubic-bezier(0.22, 1, 0.36, 1) both;
}

.flow {
  display: flex;
  gap: 7px;
  margin-bottom: 18px;
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

.title {
  margin: 0;
  font-size: 24px;
  font-weight: 800;
  letter-spacing: 0.04em;
}
.sub {
  margin: 5px 0 28px;
  color: var(--ink-soft);
  font-size: 12px;
}
.login-btn {
  width: 100%;
  letter-spacing: 0.4em;
  height: 44px;
  font-size: 15px;
}
</style>
