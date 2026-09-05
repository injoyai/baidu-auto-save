import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/login', component: () => import('../views/Login.vue') },
  {
    path: '/',
    component: () => import('../views/Layout.vue'),
    children: [
      { path: '', component: () => import('../views/Dashboard.vue'), meta: { title: '仪表盘' } },
      { path: 'accounts', component: () => import('../views/Accounts.vue'), meta: { title: '账号管理' } },
      { path: 'tasks', component: () => import('../views/Tasks.vue'), meta: { title: '任务管理' } },
      { path: 'tasks/new', component: () => import('../views/TaskEdit.vue'), meta: { title: '新建任务' } },
      { path: 'tasks/:id/edit', component: () => import('../views/TaskEdit.vue'), meta: { title: '编辑任务' } },
      { path: 'logs', component: () => import('../views/Logs.vue'), meta: { title: '转存日志' } },
      { path: 'settings', component: () => import('../views/Settings.vue'), meta: { title: '设置' } }
    ]
  }
]

const router = createRouter({ routes, history: createWebHistory() })

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.path !== '/login' && !auth.logged) return '/login'
  if (to.path === '/login' && auth.logged) return '/'
})

export default router
