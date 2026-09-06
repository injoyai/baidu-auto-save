<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'

const props = defineProps({
  modelValue: Boolean, // 弹窗显隐
  taskId: Number // null = 新建
})
const emit = defineEmits(['update:modelValue', 'saved'])

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const step = ref(0)
const accounts = ref([])
const saving = ref(false)

const blankForm = () => ({
  name: '',
  account_id: null,
  share_url: '',
  pwd: '',
  save_dir: '/来自：分享',
  folder_paths: [],
  folder_renames: {},
  folder_filter: '',
  exclude_folder_filter: '',
  regex_pattern: '',
  regex_replace: '',
  cron_expr: '0 8 * * *',
  enabled: true
})
const form = ref(blankForm())

// 打开时加载：编辑取任务回填，新建重置；重置步骤与目录树
watch(
  () => props.modelValue,
  async (open) => {
    if (!open) return
    step.value = 0
    form.value = blankForm()
    if (props.taskId) {
      const t = await api.get('/tasks').then((l) => l.find((x) => x.id === props.taskId))
      if (t) form.value = { ...form.value, ...t }
    }
    nextTick(() => rePreview())
  }
)

// 分享目录树（懒加载）
const treeRef = ref()
const treeProps = { label: 'name', children: 'children', isLeaf: (d) => !d.is_dir }

const cronPresets = [
  { label: '每小时', value: '0 * * * *' },
  { label: '每日 8 点', value: '0 8 * * *' },
  { label: '每周一 8 点', value: '0 8 * * 1' },
  { label: '仅手动执行', value: '' }
]

// cron 实时校验 + 未来触发时间预览（防抖 400ms）
const cronValid = ref(true)
const cronNext = ref([])
let cronTimer = null
watch(
  () => form.value.cron_expr,
  (expr) => {
    clearTimeout(cronTimer)
    cronTimer = setTimeout(async () => {
      const r = await api.post('/cron/preview', { expr })
      cronValid.value = r.valid
      cronNext.value = r.next || []
    }, 400)
  },
  { immediate: true }
)

async function loadNode(node, resolve) {
  // Element Plus 懒加载：根节点 level=0 也走 load，需返回分享根目录列表
  const list = await api.post('/shares/preview', {
    share_url: form.value.share_url,
    pwd: form.value.pwd,
    account_id: form.value.account_id,
    path: node.level === 0 ? '' : node.data.path
  })
  resolve(list.list)
}

function rePreview() {
  treeRef.value?.root?.childNodes?.forEach((n) => n.remove?.())
  const root = treeRef.value?.root
  if (root) root.loaded = false
  treeRef.value?.load?.()
}

const canNext = computed(() => {
  if (step.value === 0) {
    return form.value.name && form.value.share_url && form.value.account_id && form.value.save_dir
  }
  if (step.value === 3) return cronValid.value
  return true
})

async function save() {
  saving.value = true
  try {
    if (props.taskId) {
      await api.put('/tasks/' + props.taskId, form.value)
      ElMessage.success('已保存')
    } else {
      await api.post('/tasks', form.value)
      ElMessage.success('任务已创建')
    }
    visible.value = false
    emit('saved')
  } finally {
    saving.value = false
  }
}

onAccounts()
async function onAccounts() {
  accounts.value = await api.get('/accounts')
}
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="taskId ? '编辑任务' : '新建任务'"
    width="780px"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    class="task-dlg"
    destroy-on-close
  >
    <!-- 头部签名：极光三点 + 步骤指示，呼应侧栏品牌区 -->
    <template #header>
      <div class="dlg-head">
        <div class="flow" aria-hidden="true"><span></span><span></span><span></span></div>
        <span class="dlg-title">{{ taskId ? '编辑任务' : '新建任务' }}</span>
        <span class="dlg-step mono">{{ step + 1 }} / 4</span>
      </div>
    </template>

    <el-steps :active="step" simple finish-status="success" class="steps">
      <el-step title="基本信息" />
      <el-step title="选择目录" />
      <el-step title="过滤重命名" />
      <el-step title="定时" />
    </el-steps>

    <div v-show="step === 0" class="step-body">
      <el-form label-position="top">
        <el-form-item label="任务名称">
          <el-input v-model="form.name" placeholder="如：每日课程更新" />
        </el-form-item>
        <el-form-item label="分享链接">
          <el-input v-model="form.share_url" placeholder="https://pan.baidu.com/s/1xxxx?pwd=xxxx" />
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="提取码">
            <el-input v-model="form.pwd" placeholder="没有可留空" />
          </el-form-item>
          <el-form-item label="转存账号">
            <el-select v-model="form.account_id" placeholder="选择账号">
              <el-option v-for="a in accounts" :key="a.id" :value="a.id" :label="a.name + (a.status === 'invalid' ? '（失效）' : '')" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="保存目录">
          <el-input v-model="form.save_dir" placeholder="/来自：分享" />
        </el-form-item>
      </el-form>
    </div>

    <div v-show="step === 1" class="step-body">
      <el-alert v-if="!form.share_url || !form.account_id" title="请先在第一步填写分享链接并选择账号" type="info" :closable="false" />
      <el-tree
        v-if="step === 1"
        ref="treeRef"
        lazy
        :load="loadNode"
        :props="treeProps"
        show-checkbox
        node-key="path"
        :default-checked-keys="form.folder_paths"
        @check="(d, { checkedKeys }) => (form.folder_paths = checkedKeys)"
      >
        <template #default="{ data }">
          <span class="tree-row">
            <span>{{ data.is_dir ? '📁' : '📄' }} {{ data.name }}
              <el-text v-if="!data.is_dir" size="small" type="info" class="mono">（{{ (data.size / 1024 / 1024).toFixed(1) }} MB）</el-text>
            </span>
            <el-tooltip v-if="data.is_dir" content="转存后此文件夹改名为（留空 = 保持原名），对其所有子文件生效" placement="top">
              <el-input
                v-model="form.folder_renames[data.path]"
                class="rename-input"
                size="small"
                placeholder="改名…"
                @click.stop
              />
            </el-tooltip>
          </span>
        </template>
      </el-tree>
      <el-text type="info" size="small">不勾选任何目录 = 转存全部文件；文件夹「改名」仅对勾选转存的目录生效</el-text>
    </div>

    <div v-show="step === 2" class="step-body">
      <div class="form-grid">
        <el-form-item label="包含文件夹正则">
          <el-input v-model="form.folder_filter" placeholder="如：第\d+章，留空=不限" />
        </el-form-item>
        <el-form-item label="排除文件夹正则">
          <el-input v-model="form.exclude_folder_filter" placeholder="如：预告|花絮" />
        </el-form-item>
        <el-form-item label="文件名正则">
          <el-input v-model="form.regex_pattern" placeholder="如：第(\d+)课.*\.mp4，留空=全部" />
        </el-form-item>
        <el-form-item label="重命名模板">
          <el-input v-model="form.regex_replace" placeholder="如：第\1课.mp4，留空=不改名">
            <template #append><span class="mono">\1</span></template>
          </el-input>
        </el-form-item>
      </div>
    </div>

    <div v-show="step === 3" class="step-body">
      <el-form label-position="top">
        <el-form-item label="定时计划">
          <el-radio-group v-model="form.cron_expr">
            <el-radio-button v-for="p in cronPresets" :key="p.value" :value="p.value">{{ p.label }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="Cron 表达式（5 段：分 时 日 月 周，留空 = 仅手动执行）">
          <el-input
            v-model="form.cron_expr"
            placeholder="如 */10 * * * * 表示每 10 分钟"
            class="mono"
            :class="{ 'cron-invalid': !cronValid }"
            clearable
          />
          <div v-if="form.cron_expr && !cronValid" class="cron-hint cron-err">表达式非法，请检查 5 段格式</div>
          <div v-else-if="cronNext.length" class="cron-hint">
            <div class="cron-next-title">接下来 5 次运行：</div>
            <div v-for="t in cronNext" :key="t" class="mono cron-next-item">{{ t }}</div>
          </div>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
    </div>

    <template #footer>
      <div class="dlg-foot">
        <el-button v-if="step > 0" @click="step--">上一步</el-button>
        <span class="foot-spacer"></span>
        <el-button v-if="step < 3" type="primary" :disabled="!canNext" @click="step++">下一步</el-button>
        <el-button v-if="step === 3" type="primary" :loading="saving" @click="save">保存任务</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
/* 弹窗与页面卡片区分层：更大圆角 + 头部极光带 */
.task-dlg {
  border-radius: 14px;
}
.dlg-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.dlg-title {
  font-size: 16px;
  font-weight: 800;
  letter-spacing: 0.02em;
}
.dlg-step {
  margin-left: auto;
  font-size: 12px;
  color: var(--ink-soft);
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

.steps {
  margin-bottom: 18px;
  --el-border-radius-base: 8px;
}
.step-body {
  min-height: 300px;
  max-height: 60vh;
  overflow-y: auto;
}
/* 步骤 2/3 双列字段网格；窄弹窗内两列等宽 */
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  column-gap: 14px;
}
.cron-invalid :deep(.el-input__wrapper) {
  box-shadow: 0 0 0 1px var(--danger, #f56c6c) inset;
}
.cron-hint {
  width: 100%;
  margin-top: 8px;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.8;
}
.cron-err {
  color: var(--danger, #f56c6c);
}
.cron-next-title {
  color: var(--muted);
}
.cron-next-item {
  color: var(--accent, #0fa3a3);
}
.tree-row {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
  padding-right: 8px;
}
.rename-input {
  width: 150px;
  flex-shrink: 0;
}
.rename-input :deep(.el-input__inner) {
  padding: 0 8px;
}

.dlg-foot {
  display: flex;
  align-items: center;
}
.foot-spacer {
  flex: 1;
}
.dlg-foot :deep(.el-button + .el-button) {
  margin-left: 10px;
}
</style>
