<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { fmtTime } from '../utils/time'
import Reveal from '../components/Reveal.vue'

const list = ref([])
const router = useRouter()

const statusMap = {
  idle: { text: '空闲', type: 'info' },
  running: { text: '执行中', type: 'primary' },
  error: { text: '异常', type: 'danger' },
  link_invalid: { text: '链接失效', type: 'warning' }
}

async function load() {
  list.value = await api.get('/tasks')
}

async function run(task) {
  await api.post(`/tasks/${task.id}/run`)
  ElMessage.success('已触发，稍后查看日志')
  setTimeout(load, 1500)
}

async function toggle(task) {
  const r = await api.post(`/tasks/${task.id}/toggle`)
  task.enabled = r.enabled
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
      <el-button type="primary" @click="router.push('/tasks/new')">新建任务</el-button>
    </div>

  <Reveal>
    <el-card>
      <el-table :data="list">
      <el-table-column prop="name" label="名称" min-width="200">
        <template #default="{ row }">
          <span class="task-name">{{ row.name }}</span>
          <div class="task-url mono">{{ row.share_url }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="save_dir" label="保存目录" min-width="180" show-overflow-tooltip />
      <el-table-column prop="cron_expr" label="定时" width="130">
        <template #default="{ row }"><span class="mono">{{ row.cron_expr || '手动' }}</span></template>
      </el-table-column>
      <el-table-column label="状态" width="96">
        <template #default="{ row }">
          <el-tag :type="statusMap[row.status]?.type || 'info'" size="small" effect="light" round>
            {{ statusMap[row.status]?.text || row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="next_run_at" label="下次运行" width="170">
        <template #default="{ row }"><span class="mono">{{ fmtTime(row.next_run_at) }}</span></template>
      </el-table-column>
      <el-table-column label="启用" width="80" align="center">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled" @change="toggle(row)" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="240" align="right">
        <template #default="{ row }">
          <el-button size="small" type="primary" plain @click="run(row)">立即运行</el-button>
          <el-button size="small" @click="router.push(`/tasks/${row.id}/edit`)">编辑</el-button>
          <el-button size="small" type="danger" plain @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="还没有任务，点击右上角「新建任务」创建" :image-size="88" />
      </template>
    </el-table>
    </el-card>
  </Reveal>
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
</style>
