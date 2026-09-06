<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import { fmtTime } from '../utils/time'
import Reveal from '../components/Reveal.vue'

const list = ref([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
const taskFilter = ref(null)
const tasks = ref([])

const resultType = { success: 'success', skipped: 'info', failed: 'danger', partial: 'warning' }
const resultText = { success: '成功', skipped: '无新增', failed: '失败', partial: '部分成功' }

async function load() {
  const data = taskFilter.value
    ? await api.get(`/tasks/${taskFilter.value}/logs`, { params: { page: page.value, size: size.value } })
    : await api.get('/logs', { params: { page: page.value, size: size.value } })
  list.value = data.list
  total.value = data.total
}

onMounted(async () => {
  tasks.value = await api.get('/tasks')
  load()
})
</script>

<template>
  <div>
    <div class="page-head">
    <h1 class="page-title">转存日志</h1>
    <el-select
      v-model="taskFilter"
      placeholder="全部任务"
      clearable
      filterable
      style="width: 220px"
      @change="page = 1; load()"
    >
      <el-option v-for="t in tasks" :key="t.id" :value="t.id" :label="t.name" />
    </el-select>
  </div>

  <Reveal>
    <el-card>
    <el-table :data="list">
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="expand-body">摘要：{{ row.message || '（无）' }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="id" label="Run ID" min-width="90">
        <template #default="{ row }"><span class="mono">#{{ row.id }}</span></template>
      </el-table-column>
      <el-table-column prop="task_name" label="任务" min-width="160" />
      <el-table-column label="结果" min-width="100">
        <template #default="{ row }">
          <el-tag :type="resultType[row.result]" size="small" effect="light" round>{{ resultText[row.result] || row.result }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="file_count" label="新转存" min-width="90" />
      <el-table-column prop="skip_count" label="跳过" min-width="90" />
      <el-table-column prop="message" label="摘要" min-width="240" show-overflow-tooltip />
      <el-table-column prop="run_at" label="时间" min-width="160">
        <template #default="{ row }"><span class="mono">{{ fmtTime(row.run_at) }}</span></template>
      </el-table-column>
      <template #empty><el-empty description="暂无日志" :image-size="72" /></template>
    </el-table>

    <el-pagination
      v-model:current-page="page"
      v-model:page-size="size"
      :total="total"
      layout="total, prev, pager, next"
      class="pager"
      @current-change="load"
    />
    </el-card>
  </Reveal>
  </div>
</template>

<style scoped>
.pager {
  margin-top: 14px;
  justify-content: flex-end;
}
.expand-body {
  padding: 0 20px 12px;
  color: var(--ink-soft);
  font-size: 13px;
}
</style>
