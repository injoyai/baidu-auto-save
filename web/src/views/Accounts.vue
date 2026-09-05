<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { fmtTime } from '../utils/time'
import Reveal from '../components/Reveal.vue'

const list = ref([])
const dlg = ref(false)
const saving = ref(false)
const editing = ref(null) // null=新增，否则为待修复账号 id
const form = ref({ name: '', cookie: '' })

async function load() {
  list.value = await api.get('/accounts')
}

function openAdd() {
  editing.value = null
  form.value = { name: '', cookie: '' }
  dlg.value = true
}

function openFix(acc) {
  editing.value = acc.id
  form.value = { name: acc.name, cookie: '' }
  dlg.value = true
}

async function save() {
  if (!form.value.cookie.trim()) return ElMessage.warning('请粘贴 Cookie')
  saving.value = true
  try {
    if (editing.value) {
      await api.put('/accounts/' + editing.value, form.value)
      ElMessage.success('Cookie 已更新')
    } else {
      await api.post('/accounts', form.value)
      ElMessage.success('账号已添加')
    }
    dlg.value = false
    load()
  } finally {
    saving.value = false
  }
}

const checking = ref(null)
async function check(acc) {
  checking.value = acc.id
  try {
    const r = await api.post('/accounts/' + acc.id + '/check')
    ElMessage.success('Cookie 有效，容量已刷新')
    acc.quota_used = r.quota_used
    acc.quota_total = r.quota_total
    acc.status = 'active'
  } finally {
    checking.value = null
  }
}

async function remove(acc) {
  await ElMessageBox.confirm('确定删除账号「' + acc.name + '」？', '提示', { type: 'warning' })
  await api.delete('/accounts/' + acc.id)
  ElMessage.success('已删除')
  load()
}

function fmtSize(n) {
  if (!n) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
  return n.toFixed(1) + units[i]
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h1 class="page-title">账号管理</h1>
      <el-button type="primary" @click="openAdd">添加账号</el-button>
    </div>

  <Reveal>
    <el-card>
      <el-table :data="list">
        <el-table-column prop="name" label="名称" min-width="180" />
        <el-table-column label="容量" min-width="240">
          <template #default="{ row }">
            <template v-if="row.quota_total">
              <el-progress
                :percentage="Math.min(100, Math.round((row.quota_used / row.quota_total) * 100))"
                :stroke-width="8"
                :show-text="false"
              />
              <span class="mono quota-text">{{ fmtSize(row.quota_used) }} / {{ fmtSize(row.quota_total) }}</span>
            </template>
            <span v-else class="quota-text">未检查</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="96">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small" effect="light" round>
              {{ row.status === 'active' ? '正常' : '失效' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_check_at" label="最近检查" width="170">
          <template #default="{ row }">
            <span class="mono">{{ fmtTime(row.last_check_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" align="right">
          <template #default="{ row }">
            <el-button size="small" :loading="checking === row.id" @click="check(row)">检查</el-button>
            <el-button size="small" :type="row.status === 'invalid' ? 'warning' : 'default'" @click="openFix(row)">更新 Cookie</el-button>
            <el-button size="small" type="danger" plain @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="还没有账号，点击右上角「添加账号」开始" :image-size="88" />
        </template>
      </el-table>
    </el-card>
  </Reveal>

  <el-dialog v-model="dlg" :title="editing ? '更新 Cookie' : '添加账号'" width="520px">
    <el-form label-position="top">
      <el-form-item label="账号名称" v-if="!editing">
        <el-input v-model="form.name" placeholder="如：主账号" />
      </el-form-item>
      <el-form-item label="粘贴 Cookie">
        <el-input
          v-model="form.cookie"
          type="textarea"
          :rows="4"
          placeholder="在 pan.baidu.com 登录后，F12 → 网络 → 任一请求 → 请求标头 → 复制整段 Cookie，直接粘贴到这里"
          class="mono cookie-input"
        />
      </el-form-item>
    </el-form>

    <el-collapse class="help">
      <el-collapse-item title="如何获取 Cookie？">
        <ol class="help-steps">
          <li>电脑浏览器登录 <span class="mono">pan.baidu.com</span></li>
          <li>按 F12 打开开发者工具，切到「网络 / Network」</li>
          <li>刷新页面，点任意请求，在「请求标头」里找到 <span class="mono">Cookie:</span> 一行</li>
          <li>右键复制整段值，粘贴到上面的输入框——BDUSS、STOKEN 会自动识别</li>
        </ol>
      </el-collapse-item>
    </el-collapse>

    <template #footer>
      <el-button @click="dlg = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>
  </div>
</template>

<style scoped>
.quota-text {
  display: block;
  font-size: 12px;
  color: var(--ink-soft);
  margin-top: 2px;
}
.cookie-input :deep(textarea) {
  font-size: 12px;
  word-break: break-all;
}
.help {
  margin-top: 4px;
  border: none;
}
.help-steps {
  margin: 0;
  padding-left: 18px;
  color: var(--ink-soft);
  font-size: 13px;
  line-height: 2;
}
</style>
