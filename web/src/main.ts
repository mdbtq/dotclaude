import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('./views/Overview.vue') },
    { path: '/agents', component: () => import('./views/Components.vue'), props: { kind: 'agent' } },
    { path: '/skills', component: () => import('./views/Components.vue'), props: { kind: 'skill' } },
    { path: '/commands', component: () => import('./views/Components.vue'), props: { kind: 'command' } },
    { path: '/plugins', component: () => import('./views/Plugins.vue') },
    { path: '/hooks', component: () => import('./views/Hooks.vue') },
    { path: '/mcp', component: () => import('./views/Mcp.vue') },
    { path: '/memory', component: () => import('./views/Memory.vue') },
    { path: '/instructions', component: () => import('./views/Instructions.vue') },
    { path: '/settings', component: () => import('./views/Settings.vue') },
    { path: '/lint', component: () => import('./views/Lint.vue') },
  ],
})

createApp(App).use(router).mount('#app')
