<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { fmtTime } from '../utils/time'
import Reveal from '../components/Reveal.vue'
import TaskDialog from '../components/TaskDialog.vue'
import RunDialog from '../components/RunDialog.vue'

const list = ref([])

const statusMap = {
  idle: { text: '空闲', type: 'info' },
  running: { text: '执行中', type: 'primary' },
  error: { text: '异常', type: 'danger' },
  link_invalid: { text: '链接失效', type: 'warning' }
}

// 弹窗状态：null=关闭，{id}=编辑，{id:null}=新建
const dlg = ref(false)
const editingId = ref(null)

// 运行详情弹窗：{id, name} 或 null
const runDlgTask = ref(null)
const runDlgOpen = computed({
  get: () => runDlgTask.value != null,
  set: (v) => { if (!v) runDlgTask.value = null }
})

function openNew() {
  editingId.value = null
  dlg.value = true
}
function openEdit(row) {
  editingId.value = row.id
  dlg.value = true
}
function openRun(row) {
  runDlgTask.value = { id: row.id, name: row.name }
}

async function load() {
  list.value = await api.get('/tasks')
}

async function run(task) {
  await api.post(`/tasks/${task.id}/run`)
  ElMessage.success('已触发')
  // 运行态由后端异步落库，稍等再刷新让状态变「执行中」
  setTimeout(load, 600)
  openRun(task)
}

async function toggle(task) {
  await api.post(`/tasks/${task.id}/toggle`)
  // 后端同时更新了 next_run_at 等字段，重载列表保证整行数据同步
  await load()
}

async function remove(task) {
  await ElMessageBox.confirm('删除任务会同时清除其转存记录，确定？', '提示', { type: 'warning' })
  await api.delete('/tasks/' + task.id)
  load()
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h1 class="page-title">任务管理</h1>
      <el-button type="primary" @click="openNew">新建任务</el-button>
    </div>

  <Reveal>
    <el-card>
      <el-table :data="list">
      <el-table-column prop="name" label="名称" min-width="220">
        <template #default="{ row }">
          <span class="task-name">{{ row.name }}</span>
          <div class="task-url mono">{{ row.share_url }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="save_dir" label="保存目录" min-width="180" show-overflow-tooltip />
      <el-table-column prop="cron_expr" label="定时" min-width="110">
        <template #default="{ row }"><span class="mono">{{ row.cron_expr || '手动' }}</span></template>
      </el-table-column>
      <el-table-column label="状态" min-width="100">
        <template #default="{ row }">
          <el-tag
            v-if="row.status === 'running'"
            type="primary"
            size="small"
            effect="light"
            round
            class="run-tag"
            @click="openRun(row)"
          >执行中</el-tag>
          <el-tag v-else :type="statusMap[row.status]?.type || 'info'" size="small" effect="light" round>
            {{ statusMap[row.status]?.text || row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="next_run_at" label="下次运行" min-width="160">
        <template #default="{ row }"><span class="mono">{{ fmtTime(row.next_run_at) }}</span></template>
      </el-table-column>
      <el-table-column label="启用" min-width="90" align="center">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled" @change="toggle(row)" />
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="280" align="right">
        <template #default="{ row }">
          <el-button
            size="small"
            type="primary"
            plain
            :disabled="row.status === 'running'"
            @click="run(row)"
          >立即运行</el-button>
          <el-button
            size="small"
            :type="row.status === 'running' ? 'primary' : 'default'"
            @click="openRun(row)"
          >详情</el-button>
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" plain :disabled="row.status === 'running'" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="还没有任务，点击右上角「新建任务」创建" :image-size="88" />
      </template>
    </el-table>
    </el-card>
  </Reveal>

  <TaskDialog v-model="dlg" :task-id="editingId" @saved="load" />
  <RunDialog
    v-if="runDlgTask"
    v-model="runDlgOpen"
    :task="runDlgTask"
    @finished="load"
  />
  </div>
</template>

<style scoped>
.task-name {
  font-weight: 600;
}
.task-url {
  font-size: 11px;
  color: var(--ink-soft);
  margin-top: 2px;
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
/* 「执行中」状态标签可点击，手型指针提示 */
.run-tag {
  cursor: pointer;
}
</style>
