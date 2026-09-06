<script setup>
import { ref, computed, watch, onBeforeUnmount, nextTick } from 'vue'
import api from '../api'

const props = defineProps({
  modelValue: Boolean,
  task: { type: Object, default: null } // {id, name}
})
const emit = defineEmits(['update:modelValue', 'finished']) // finished: 运行结束后通知列表刷新

const visible = ref(false)
const stage = ref('')
const done = ref(0)
const total = ref(0)
const logs = ref([])
const running = ref(false)
const result = ref('') // success | skipped | failed（结束后）
const resultMsg = ref('')
const logBox = ref(null)
let timer = null
// 是否已经通知过父级（finished 事件只发一次）
let notified = false
// 后端快照是否存在（重启后无快照，结果标签避免误显示「成功」）
const hasSnapshot = ref(false)
// 已结束（快照标记结束，或后端已不在运行且无快照）
const finished = computed(() => hasSnapshot.value && !running.value)

// 是否处于「转存中」阶段（有量化进度）；其他阶段无进度可言
const transferPhase = computed(() => hasSnapshot.value && total.value > 0)
const pct = computed(() => total.value > 0 ? Math.min(100, Math.round((done.value / total.value) * 100)) : 0)

const resultText = computed(() => {
  if (!finished.value) return '已结束'
  return { success: '成功', skipped: '无新增', failed: '失败' }[result.value] || '已结束'
})
const resultType = computed(() => {
  if (!finished.value) return 'info'
  return { success: 'success', skipped: 'info', failed: 'danger' }[result.value] || 'info'
})

watch(() => props.modelValue, (v) => {
  visible.value = v
  if (v) {
    poll()
  } else {
    stopPoll()
  }
}, { immediate: true }) // 组件挂载时 modelValue 已为 true（v-if 创建即打开），必须立即执行
watch(visible, (v) => emit('update:modelValue', v))

async function poll() {
  try {
    const d = await api.get(`/tasks/${props.task.id}/status`)
    hasSnapshot.value = d.stage !== '' || (d.logs && d.logs.length > 0) || !!d.result
    stage.value = d.stage
    done.value = d.done
    total.value = d.total
    logs.value = d.logs
    running.value = !!d.running
    result.value = d.result || ''
    resultMsg.value = d.resultMsg || ''
    if (!d.running) {
      // 运行结束（或后端无快照如刚重启）：停轮询；有快照时通知列表刷新一次
      stopPoll()
      if (!notified) {
        notified = true
        emit('finished')
      }
    }
  } catch {
    /* 轮询失败静默，下轮重试 */
  }
  if (timer) return
  timer = setInterval(poll, 1500)
}

function stopPoll() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

// 日志自动滚到底部
function scrollLog() {
  nextTick(() => {
    if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
  })
}
watch(logs, scrollLog, { deep: true })

onBeforeUnmount(stopPoll)
</script>

<template>
  <el-dialog
    v-model="visible"
    width="640px"
    :close-on-click-modal="false"
    class="run-dlg"
    destroy-on-close
  >
    <template #header>
      <div class="dlg-head">
        <div class="flow" aria-hidden="true"><span></span><span></span><span></span></div>
        <span class="dlg-title">运行详情 · {{ task?.name }}</span>
        <el-tag v-if="running" type="primary" size="small" effect="light" round>执行中</el-tag>
        <el-tag v-else :type="resultType" size="small" effect="light" round>{{ resultText }}</el-tag>
      </div>
    </template>

    <!-- 阶段 + 进度：仅在「转存中」阶段显示真实 done/total，
         其余阶段（访问/验证/遍历/去重）无量化进度，用流动动画表达「进行中」不显示数字 -->
    <div class="run-progress">
      <div class="stage-line">
        <span class="stage">{{ finished ? `完成：${resultText}` : (stage || (running ? '等待执行' : '—')) }}</span>
        <span v-if="transferPhase" class="mono count">{{ done }} / {{ total }}</span>
      </div>
      <el-progress
        v-if="transferPhase"
        :percentage="pct"
        :status="finished ? (result === 'failed' ? 'exception' : 'success') : ''"
        :stroke-width="10"
      />
      <el-progress
        v-else-if="running"
        :percentage="50"
        indeterminate
        :duration="3"
        :show-text="false"
        :stroke-width="10"
      />
    </div>

    <!-- 运行日志（结束后保留，可随时回看最后一次） -->
    <div ref="logBox" class="log-box mono">
      <div v-if="logs.length === 0" class="log-empty">
        {{ running ? '等待日志输出…' : '暂无运行记录（重启服务后只保留历史转存日志页的数据）' }}
      </div>
      <div v-for="(l, i) in logs" :key="i" class="log-line">{{ l }}</div>
    </div>
  </el-dialog>
</template>

<style scoped>
.dlg-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.dlg-title {
  font-weight: 600;
}
.flow {
  display: flex;
  gap: 5px;
}
.flow span {
  width: 10px; height: 10px;
  border-radius: 3px;
  animation: drift 2.2s ease-in-out infinite;
}
.flow span:nth-child(1) { background: #14b8a6; }
.flow span:nth-child(2) { background: #38bdf8; animation-delay: 0.25s; opacity: 0.8; }
.flow span:nth-child(3) { background: #8b5cf6; animation-delay: 0.5s; opacity: 0.6; }
@keyframes drift {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-5px); }
}

.run-progress {
  margin-bottom: 16px;
}
.stage-line {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 8px;
}
.stage {
  font-weight: 600;
}
.count {
  font-size: 12px;
  color: var(--ink-soft);
}

.log-box {
  height: 320px;
  overflow-y: auto;
  background: #0d1522;
  border-radius: 8px;
  padding: 12px 14px;
  font-size: 12px;
  line-height: 1.7;
  color: #9fd3c7;
}
.log-line {
  white-space: pre-wrap;
  word-break: break-all;
}
.log-empty {
  color: #5a7184;
}
</style>
