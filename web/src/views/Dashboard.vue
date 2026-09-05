<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import { fmtTime } from '../utils/time'
import AnimNumber from '../components/AnimNumber.vue'
import Reveal from '../components/Reveal.vue'

const data = ref(null)

function fmtSize(n) {
  if (!n) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
  return n.toFixed(1) + units[i]
}

const statusText = { idle: '空闲', running: '执行中', error: '异常', link_invalid: '链接失效' }
const resultType = { success: 'success', skipped: 'info', failed: 'danger', partial: 'warning' }

onMounted(async () => {
  data.value = await api.get('/dashboard')
})
</script>

<template>
  <div>
    <div class="page-head">
      <h1 class="page-title">仪表盘</h1>
    </div>

  <div v-if="data">
    <!-- 账号容量：瀑布入场 -->
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12" :md="8" v-for="(acc, i) in data.accounts" :key="acc.id">
        <Reveal :style="{ '--reveal-delay': i * 0.08 + 's' }">
          <el-card class="acc-card">
            <div class="acc-head">
              <span class="acc-name">{{ acc.name }}</span>
              <el-tag :type="acc.status === 'active' ? 'success' : 'danger'" size="small" effect="light" round>
                {{ acc.status === 'active' ? '正常' : 'Cookie 失效' }}
              </el-tag>
            </div>
            <el-progress
              :percentage="acc.quota_total ? Math.min(100, Math.round((acc.quota_used / acc.quota_total) * 100)) : 0"
              :stroke-width="9"
            />
            <div class="mono quota">{{ fmtSize(acc.quota_used) }} / {{ fmtSize(acc.quota_total) }}</div>
          </el-card>
        </Reveal>
      </el-col>
    </el-row>

    <!-- 统计：数字滚动 -->
    <el-row :gutter="16" class="stat-row">
      <el-col :xs="24" :md="8">
        <Reveal :style="{ '--reveal-delay': '0.1s' }">
          <el-card class="stat-card">
            <div class="stat-num mono"><AnimNumber :value="data.recent_7d_files" /></div>
            <div class="stat-label">近 7 日转存文件</div>
          </el-card>
        </Reveal>
      </el-col>
      <el-col :xs="24" :md="16">
        <Reveal :style="{ '--reveal-delay': '0.18s' }">
          <el-card class="stat-card">
            <div class="stat-label">任务状态分布</div>
            <div class="chips">
              <template v-if="Object.keys(data.task_status).length">
                <div v-for="(n, s) in data.task_status" :key="s" class="chip">
                  <span class="chip-num mono"><AnimNumber :value="n" /></span>
                  <span class="chip-label">{{ statusText[s] || s }}</span>
                </div>
              </template>
              <el-text v-else type="info">暂无任务，去「任务管理」创建第一个</el-text>
            </div>
          </el-card>
        </Reveal>
      </el-col>
    </el-row>

    <!-- 最近转存 -->
    <Reveal :style="{ '--reveal-delay': '0.26s' }">
      <el-card class="recent">
        <template #header>最近转存记录</template>
        <el-table :data="data.recent_logs" size="small">
          <el-table-column prop="id" label="Run ID" width="90">
            <template #default="{ row }"><span class="mono">#{{ row.id }}</span></template>
          </el-table-column>
          <el-table-column prop="task_name" label="任务" min-width="160" />
          <el-table-column prop="result" label="结果" width="96">
            <template #default="{ row }">
              <el-tag :type="resultType[row.result]" size="small" effect="light" round>{{ row.result }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="file_count" label="新转存" width="84" align="center" />
          <el-table-column prop="skip_count" label="跳过" width="84" align="center" />
          <el-table-column prop="message" label="摘要" min-width="240" show-overflow-tooltip />
          <el-table-column prop="run_at" label="时间" width="170">
            <template #default="{ row }"><span class="mono">{{ fmtTime(row.run_at) }}</span></template>
          </el-table-column>
          <template #empty><el-empty description="还没有转存记录" :image-size="72" /></template>
        </el-table>
      </el-card>
    </Reveal>
  </div>
  </div>
</template>

<style scoped>
.acc-card :deep(.el-card__body) {
  padding: 18px 20px 14px;
}
.acc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.acc-name {
  font-weight: 700;
}
.quota {
  font-size: 12px;
  color: var(--ink-soft);
  margin-top: 4px;
}
.stat-row {
  margin-top: 16px;
}
.stat-card :deep(.el-card__body) {
  padding: 20px 22px;
}
.stat-num {
  font-size: 38px;
  font-weight: 800;
  line-height: 1.1;
}
.stat-label {
  color: var(--ink-soft);
  font-size: 13px;
  margin-top: 6px;
}
.chips {
  display: flex;
  gap: 26px;
  margin-top: 12px;
  flex-wrap: wrap;
}
.chip-num {
  font-size: 26px;
  font-weight: 800;
  display: block;
}
.chip-label {
  font-size: 12px;
  color: var(--ink-soft);
}
.recent {
  margin-top: 16px;
}
</style>
