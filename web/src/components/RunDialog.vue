<script setup>
import { ref, watch, onBeforeUnmount, nextTick } from 'vue'
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
const logBox = ref(null)
let timer = null

watch(() => props.modelValue, (v) => {
  visible.value = v
  if (v) {
    poll()
  } else {
    stopPoll()
  }
})
watch(visible, (v) => emit('update:modelValue', v))

async function poll() {
  try {
    const d = await api.get(`/tasks/${props.task.id}/status`)
    stage.value = d.stage
    done.value = d.done
    total.value = d.total
    logs.value = d.logs
    running.value = !!d.running
    // 进度条百分比：Total=0（未到计数阶段）时用不确定动画
    if (!d.running && d.finished !== false) {
      // 快照已过期或已结束：停轮询，通知父级刷新列表
      stopPoll()
      emit('finished')
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
        <el-tag v-else type="success" size="small" effect="light" round>已结束</el-tag>
      </div>
    </template>

    <!-- 阶段 + 进度 -->
    <div class="run-progress">
      <div class="stage-line">
        <span class="stage">{{ stage || (running ? '等待执行' : '—') }}</span>
        <span class="mono count">{{ total > 0 ? `${done} / ${total}` : '' }}</span>
      </div>
      <el-progress
        :percentage="total > 0 ? Math.round((done / total) * 100) : 50"
        :indeterminate="total === 0 && running"
        :status="running ? '' : 'success'"
        :stroke-width="10"
        :show-text="total > 0"
      />
    </div>

    <!-- 实时日志 -->
    <div ref="logBox" class="log-box mono">
      <div v-if="logs.length === 0" class="log-empty">等待日志输出…</div>
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
