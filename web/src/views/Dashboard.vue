<script setup>
import { ref, computed, onMounted } from 'vue'
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

// 派生指标：任务总数 / 执行中 / 失效账号数
const stat = computed(() => {
  if (!data.value) return null
  const ts = data.value.task_status || {}
  const accounts = data.value.accounts || []
  return {
    totalTasks: Object.values(ts).reduce((a, b) => a + b, 0),
    running: ts.running || 0,
    invalidAcc: accounts.filter((a) => a.status !== 'active').length
  }
})

onMounted(async () => {
  data.value = await api.get('/dashboard')
})
</script>

<template>
  <div>
    <div class="page-head">
      <h1 class="page-title">仪表盘</h1>
    </div>

  <div v-if="data && stat" class="dash">
    <!-- KPI 指标行：4 等分，窄屏降为 2 列/1 列 -->
    <div class="kpi-grid">
      <Reveal>
        <el-card class="kpi" style="--accent: var(--aurora-teal)">
          <div class="kpi-num mono grad-text"><AnimNumber :value="data.recent_7d_files" /></div>
          <div class="kpi-label">近 7 日转存文件</div>
          <div class="kpi-sub">转存成功的文件总数</div>
        </el-card>
      </Reveal>
      <Reveal style="--reveal-delay: 0.06s">
        <el-card class="kpi" style="--accent: var(--aurora-sky)">
          <div class="kpi-num mono"><AnimNumber :value="stat.totalTasks" /></div>
          <div class="kpi-label">任务总数</div>
          <div class="kpi-sub">
            <template v-if="stat.totalTasks">
              <span v-for="(n, s) in data.task_status" :key="s" class="sub-chip">
                {{ statusText[s] || s }} {{ n }}
              </span>
            </template>
            <span v-else class="sub-chip">暂无任务</span>
          </div>
        </el-card>
      </Reveal>
      <Reveal style="--reveal-delay: 0.12s">
        <el-card class="kpi" style="--accent: var(--aurora-violet)">
          <div class="kpi-num mono"><AnimNumber :value="stat.running" /></div>
          <div class="kpi-label">执行中任务</div>
          <div class="kpi-sub">{{ stat.running ? '正在转存' : '当前空闲' }}</div>
        </el-card>
      </Reveal>
      <Reveal style="--reveal-delay: 0.18s">
        <el-card
          class="kpi"
          :style="{ '--accent': stat.invalidAcc ? 'var(--danger, #f56c6c)' : 'var(--aurora-sky)' }"
        >
          <div class="kpi-num mono"><AnimNumber :value="data.accounts.length" /></div>
          <div class="kpi-label">转存账号</div>
          <div class="kpi-sub" :class="{ 'sub-warn': stat.invalidAcc }">
            {{ stat.invalidAcc ? `${stat.invalidAcc} 个 Cookie 失效，需更新` : '全部有效' }}
          </div>
        </el-card>
      </Reveal>
    </div>

    <!-- 账号容量：auto-fit 网格，1~N 个账号都均匀铺满 -->
    <div v-if="data.accounts?.length" class="acc-grid">
      <Reveal
        v-for="(acc, i) in data.accounts"
        :key="acc.id"
        :style="{ '--reveal-delay': 0.22 + i * 0.05 + 's' }"
      >
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
    </div>

    <!-- 最近转存 -->
    <Reveal style="--reveal-delay: 0.28s">
      <el-card class="recent">
        <template #header>最近转存记录</template>
        <el-table :data="data.recent_logs" size="small">
          <el-table-column prop="id" label="Run ID" min-width="90">
            <template #default="{ row }"><span class="mono">#{{ row.id }}</span></template>
          </el-table-column>
          <el-table-column prop="task_name" label="任务" min-width="160" />
          <el-table-column prop="result" label="结果" min-width="100">
            <template #default="{ row }">
              <el-tag :type="resultType[row.result]" size="small" effect="light" round>{{ row.result }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="file_count" label="新转存" min-width="90" />
          <el-table-column prop="skip_count" label="跳过" min-width="90" />
          <el-table-column prop="message" label="摘要" min-width="240" show-overflow-tooltip />
          <el-table-column prop="run_at" label="时间" min-width="160">
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
.dash {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* KPI 行：顶部彩色渐变发丝线区分指标 */
.kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  align-items: stretch;
}
.kpi-grid :deep(.reveal),
.kpi {
  height: 100%;
}
.kpi {
  position: relative;
  overflow: hidden;
}
.kpi::before {
  content: '';
  position: absolute;
  inset: 0 0 auto 0;
  height: 3px;
  background: linear-gradient(90deg, var(--accent), transparent 85%);
}
.kpi :deep(.el-card__body) {
  padding: 18px 20px 16px;
}
.kpi-num {
  font-size: 30px;
  font-weight: 800;
  line-height: 1.15;
}
.kpi-label {
  font-size: 12.5px;
  color: var(--ink-soft);
  margin-top: 4px;
}
.kpi-sub {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
  font-size: 12px;
  color: var(--ink-soft);
}
.sub-chip {
  background: #edf2f8;
  border-radius: 999px;
  padding: 1px 9px;
}
.sub-warn {
  color: var(--danger, #f56c6c);
}

/* 账号容量：auto-fit，任意数量均铺满一行 */
.acc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 16px;
}
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

@media (max-width: 1100px) {
  .kpi-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 560px) {
  .kpi-grid {
    grid-template-columns: 1fr;
  }
}
</style>
