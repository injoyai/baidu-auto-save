<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import Reveal from '../components/Reveal.vue'

const settings = ref({})
const saving = ref(false)
const testing = ref(false)

// 渠道定义：keys 为判断「已配置」的依据，fields 为该渠道的输入项
const channels = [
  {
    name: '企业微信机器人',
    keys: ['notify.wecom_webhook'],
    fields: [
      { key: 'notify.wecom_webhook', label: 'Webhook 地址', placeholder: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx', wide: true }
    ]
  },
  {
    name: 'Server酱',
    keys: ['notify.serverchan_sendkey'],
    fields: [
      { key: 'notify.serverchan_sendkey', label: 'SendKey', placeholder: 'SCTxxxxxxx', wide: true }
    ]
  },
  {
    name: 'Telegram',
    keys: ['notify.telegram_bot_token', 'notify.telegram_chat_id'],
    all: true, // 需要同时配置 Token 和 Chat ID 才生效
    fields: [
      { key: 'notify.telegram_bot_token', label: 'Bot Token', placeholder: '123456:ABC-xxx' },
      { key: 'notify.telegram_chat_id', label: 'Chat ID', placeholder: 'chat id' }
    ]
  },
  {
    name: '自定义 Webhook',
    keys: ['notify.custom_webhook'],
    fields: [
      { key: 'notify.custom_webhook', label: 'Webhook 地址（POST JSON）', placeholder: 'https://...', wide: true }
    ]
  }
]

// 渠道是否已配置（all=true 时要求全部键非空，否则任一即可）
function channelOn(ch) {
  const vals = ch.keys.map(k => (settings.value[k] || '').trim() !== '')
  return ch.all ? vals.every(Boolean) : vals.some(Boolean)
}
const anyChannel = computed(() => channels.some(channelOn))

// 通知事件：settings 存逗号分隔字符串，'' = 全部（默认），'none' = 全不推
const EVENTS = [
  { value: 'success', label: '转存成功' },
  { value: 'failed', label: '转存失败' },
  { value: 'cookie_invalid', label: 'Cookie 失效' }
]
const eventList = computed({
  get() {
    const s = (settings.value['notify.events'] || '').trim()
    if (!s || s === 'none') return s === 'none' ? [] : EVENTS.map(e => e.value)
    return s.split(',').map(x => x.trim()).filter(Boolean)
  },
  set(arr) {
    if (arr.length === EVENTS.length) settings.value['notify.events'] = ''
    else if (arr.length === 0) settings.value['notify.events'] = 'none'
    else settings.value['notify.events'] = arr.join(',')
  }
})

// 开放推送 Token：后端只回「已设置」占位，输入框显示空 + 标记，避免占位串被当成值
const token = computed({
  get: () => (settings.value['open_api_token'] === '已设置' ? '' : settings.value['open_api_token'] || ''),
  set: v => { settings.value['open_api_token'] = v }
})
const tokenSaved = computed(() => settings.value['open_api_token'] === '已设置')

onMounted(async () => {
  settings.value = await api.get('/settings')
})

async function save() {
  saving.value = true
  try {
    await api.put('/settings', settings.value)
    ElMessage.success('已保存')
    // 重新拉取，让 Token 等占位值恢复服务端状态
    settings.value = await api.get('/settings')
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
      <div class="head-actions">
        <el-button :loading="testing" :disabled="saving" @click="testNotify">发送测试通知</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存设置</el-button>
      </div>
    </div>

    <div class="settings">
      <Reveal style="--reveal-delay: 0.05s">
        <el-card class="form-card">
          <template #header>
            <div class="card-head">
              <span class="card-title">通知渠道</span>
              <el-tag :type="anyChannel ? 'success' : 'info'" size="small" effect="light" round>
                {{ anyChannel ? '已启用' : '未启用' }}
              </el-tag>
              <span class="card-sub">转存完成后按事件推送结果，可同时启用多个渠道</span>
            </div>
          </template>

          <div
            v-for="(ch, i) in channels"
            :key="ch.name"
            class="channel"
            :class="{ 'ch-first': i === 0 }"
          >
            <div class="ch-head">
              <span class="ch-name">{{ ch.name }}</span>
              <el-tag :type="channelOn(ch) ? 'success' : 'info'" size="small" effect="plain" round>
                {{ channelOn(ch) ? '已配置' : '未配置' }}
              </el-tag>
            </div>
            <el-form label-position="top" class="grid-form">
              <el-form-item
                v-for="f in ch.fields"
                :key="f.key"
                :label="f.label"
                :class="{ span2: f.wide }"
              >
                <el-input v-model="settings[f.key]" :placeholder="f.placeholder" clearable />
              </el-form-item>
            </el-form>
          </div>

          <el-form label-position="top" class="events-form">
            <el-form-item label="通知事件">
              <el-checkbox-group v-model="eventList" class="event-group">
                <el-checkbox v-for="e in EVENTS" :key="e.value" :value="e.value">{{ e.label }}</el-checkbox>
              </el-checkbox-group>
              <div class="field-hint">全部勾选（默认）= 推送所有事件；全部取消 = 不推送</div>
            </el-form-item>
          </el-form>
        </el-card>
      </Reveal>

      <Reveal style="--reveal-delay: 0.12s">
        <el-card class="form-card">
          <template #header>
            <div class="card-head">
              <span class="card-title">通用设置</span>
              <span class="card-sub">全局默认值，任务里未单独设置的项会用到</span>
            </div>
          </template>
          <el-form label-position="top" class="grid-form">
            <el-form-item label="开放推送 Token（X-API-Token）">
              <el-input v-model="token" placeholder="留空禁用开放推送" clearable>
                <template v-if="tokenSaved" #suffix>
                  <span class="tok-ok">已保存</span>
                </template>
              </el-input>
              <div class="field-hint">已保存的 Token 不回显；输入新值可更换，清空后保存即禁用</div>
            </el-form-item>
            <el-form-item label="全局默认保存目录">
              <el-input v-model="settings['global.save_dir']" placeholder="/来自：分享" clearable />
              <div class="field-hint">新建任务未单独指定目录时使用</div>
            </el-form-item>
          </el-form>
        </el-card>
      </Reveal>
    </div>
  </div>
</template>

<style scoped>
/* 顶部操作区与其他页面一致（标题左、按钮右） */
.head-actions {
  display: flex;
  gap: 4px;
}

.settings {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.form-card :deep(.el-card__body) {
  padding: 20px 22px 6px;
}
.card-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}
.card-title {
  font-size: 14.5px;
  font-weight: 700;
}
.card-sub {
  font-size: 12px;
  font-weight: 400;
  color: var(--ink-soft);
}

/* 渠道分区：名称 + 状态标签 + 字段，分区之间细分隔线 */
.channel {
  padding: 2px 0 6px;
  margin-bottom: 14px;
  border-bottom: 1px dashed var(--line);
}
.channel:last-of-type {
  border-bottom: none;
  margin-bottom: 4px;
}
.ch-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.ch-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--ink);
}
.ch-name::before {
  content: '';
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--grad);
  margin-right: 7px;
  vertical-align: 1px;
}

/* 双列表单网格 */
.grid-form {
  display: grid;
  grid-template-columns: 1fr 1fr;
  column-gap: 16px;
}
.grid-form :deep(.el-form-item) {
  margin-bottom: 14px;
}
.grid-form :deep(.span2) {
  grid-column: span 2;
}

.events-form {
  border-top: 1px dashed var(--line);
  padding-top: 14px;
}
.event-group {
  display: flex;
  gap: 4px;
}
.field-hint {
  font-size: 12px;
  line-height: 1.6;
  color: var(--ink-soft);
  margin-top: 2px;
}
.tok-ok {
  font-size: 12px;
  color: var(--aurora-teal);
  padding-right: 2px;
}

@media (max-width: 720px) {
  .grid-form {
    grid-template-columns: 1fr;
  }
  .grid-form :deep(.span2) {
    grid-column: span 1;
  }
  .head-actions {
    width: 100%;
  }
}
</style>
