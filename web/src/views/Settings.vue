<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import Reveal from '../components/Reveal.vue'

const settings = ref({})
const saving = ref(false)
const testing = ref(false)

// wide=true 的字段在双列表单中独占整行
const notifyFields = [
  { key: 'notify.wecom_webhook', label: '企业微信机器人 Webhook', placeholder: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx', wide: true },
  { key: 'notify.serverchan_sendkey', label: 'Server酱 SendKey', placeholder: 'SCTxxxxxxx' },
  { key: 'notify.telegram_bot_token', label: 'Telegram Bot Token', placeholder: '123456:ABC-xxx' },
  { key: 'notify.telegram_chat_id', label: 'Telegram Chat ID', placeholder: 'chat id' },
  { key: 'notify.custom_webhook', label: '自定义 Webhook（POST JSON）', placeholder: 'https://...', wide: true },
  { key: 'notify.events', label: '通知事件', placeholder: 'success,failed,cookie_invalid（逗号分隔，留空=全部）', wide: true }
]
const otherFields = [
  { key: 'open_api_token', label: '开放推送 Token（X-API-Token）', placeholder: '留空禁用开放推送' },
  { key: 'global.save_dir', label: '全局默认保存目录', placeholder: '/来自：分享' }
]

onMounted(async () => {
  settings.value = await api.get('/settings')
})

async function save() {
  saving.value = true
  try {
    await api.put('/settings', settings.value)
    ElMessage.success('已保存')
  } finally {
    saving.value = false
  }
}

async function testNotify() {
  testing.value = true
  try {
    await save()
    await api.post('/settings/notify/test')
    ElMessage.info('测试通知已发送，请查收')
  } finally {
    testing.value = false
  }
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1 class="page-title">设置</h1>
    </div>

    <div class="settings-layout">
      <div class="settings-main">
        <Reveal :style="{ '--reveal-delay': '0.05s' }">
          <el-card class="form-card">
            <template #header>
              <div class="card-head">
                <span>通知渠道</span>
                <span class="card-sub">转存完成后按下面的事件推送结果</span>
              </div>
            </template>
            <el-form label-position="top" class="grid-form">
              <el-form-item
                v-for="f in notifyFields"
                :key="f.key"
                :label="f.label"
                :class="{ span2: f.wide }"
              >
                <el-input v-model="settings[f.key]" :placeholder="f.placeholder" />
              </el-form-item>
            </el-form>
          </el-card>
        </Reveal>

        <Reveal :style="{ '--reveal-delay': '0.12s' }">
          <el-card class="form-card">
            <template #header>
              <div class="card-head">
                <span>开放推送与其他</span>
                <span class="card-sub">全局默认值，任务里未单独设置的项会用到</span>
              </div>
            </template>
            <el-form label-position="top" class="grid-form">
              <el-form-item v-for="f in otherFields" :key="f.key" :label="f.label">
                <el-input v-model="settings[f.key]" :placeholder="f.placeholder" />
              </el-form-item>
            </el-form>
          </el-card>
        </Reveal>
      </div>

      <div class="settings-side">
        <Reveal :style="{ '--reveal-delay': '0.18s' }">
          <el-card class="side-card">
            <div class="side-title">保存更改</div>
            <el-button
              type="primary"
              class="side-btn"
              :loading="saving"
              @click="save"
            >保存设置</el-button>
            <el-button
              class="side-btn"
              :loading="testing"
              :disabled="saving"
              @click="testNotify"
            >发送测试通知</el-button>
            <p class="side-hint">测试通知会先保存当前设置，再向已配置的渠道各发一条消息。</p>
          </el-card>
        </Reveal>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 左主列 + 右操作栏；窄屏退化为单列 */
.settings-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 264px;
  gap: 18px;
  align-items: start;
}
.settings-main {
  min-width: 0;
}
.form-card {
  margin-bottom: 18px;
}
.form-card :deep(.el-card__body) {
  padding: 20px 22px 4px;
}
.card-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}
.card-sub {
  font-size: 12px;
  font-weight: 400;
  color: var(--ink-soft);
}
/* 双列表单网格 */
.grid-form {
  display: grid;
  grid-template-columns: 1fr 1fr;
  column-gap: 16px;
}
.grid-form :deep(.el-form-item) {
  margin-bottom: 18px;
}
.grid-form :deep(.span2) {
  grid-column: span 2;
}
@media (max-width: 720px) {
  .grid-form {
    grid-template-columns: 1fr;
  }
  .grid-form :deep(.span2) {
    grid-column: span 1;
  }
}

/* 右侧粘性操作卡 */
.settings-side {
  position: sticky;
  top: 84px; /* 顶栏 58px + 间距 */
}
.side-card :deep(.el-card__body) {
  padding: 20px;
}
.side-title {
  font-size: 14px;
  font-weight: 700;
  margin-bottom: 14px;
}
.side-btn {
  width: 100%;
  margin: 0 0 10px;
}
.side-btn + .side-btn {
  margin-left: 0;
}
.side-hint {
  margin: 12px 0 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--ink-soft);
}

@media (max-width: 960px) {
  .settings-layout {
    grid-template-columns: 1fr;
  }
  .settings-side {
    position: static;
  }
  .side-btn {
    width: auto;
    margin-right: 10px;
  }
  .side-btn + .side-btn {
    margin-left: 0;
  }
}
</style>
