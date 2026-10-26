<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { pendingFix, tilde, useConfig } from './store'
import ProjectPicker from './components/ProjectPicker.vue'
import FixDialog from './components/FixDialog.vue'

const { project, config, findings, loading, error, refresh } = useConfig()
const route = useRoute()
const router = useRouter()
const menuOpen = ref(false)

const count = (kind: string) =>
  config.value?.components.filter((c) => c.kind === kind && c.status === 'active').length ?? 0

const nav = computed(() => {
  const c = config.value
  const errors = findings.value.filter((f) => f.severity === 'error').length
  const warnings = findings.value.filter((f) => f.severity === 'warning').length
  return [
    { to: '/', label: 'Overview' },
    { to: '/instructions', label: 'Instructions', count: c?.instructions.length },
    { to: '/settings', label: 'Settings', count: c?.settings.length },
    { section: 'Components' },
    { to: '/agents', label: 'Agents', count: count('agent') },
    { to: '/skills', label: 'Skills', count: count('skill') },
    { to: '/commands', label: 'Commands', count: count('command') },
    { to: '/plugins', label: 'Plugins', count: c?.plugins?.filter((p) => p.enabled).length },
    { to: '/hooks', label: 'Hooks', count: c?.hooks?.filter((h) => h.status === 'active').length },
    { to: '/mcp', label: 'MCP servers', count: c?.mcpServers?.filter((s) => s.status === 'active').length },
    { section: 'Knowledge' },
    { to: '/memory', label: 'Memory', count: c?.memory?.reduce((n, s) => n + s.entries.length, 0) },
    { section: 'Health' },
    { to: '/lint', label: 'Lint', count: errors + warnings, alert: errors > 0 },
  ]
})

function linkTo(path: string) {
  return { path, query: project.value ? { project: project.value } : {} }
}

function selectProject(p: string) {
  router.push({ path: route.path, query: p ? { project: p } : {} })
}
</script>

<template>
  <div class="flex min-h-screen">
    <!-- Sidebar -->
    <aside
      class="fixed inset-y-0 left-0 z-30 w-60 shrink-0 border-r border-zinc-200 bg-white transition-transform dark:border-zinc-800 dark:bg-zinc-900 lg:sticky lg:top-0 lg:h-screen lg:translate-x-0"
      :class="menuOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="flex h-14 items-center gap-2 px-5">
        <span class="grid size-7 place-items-center rounded-md bg-accent-600 font-mono text-[13px] font-bold text-white">.c</span>
        <span class="font-semibold tracking-tight">dotclaude</span>
      </div>
      <nav class="space-y-0.5 px-3 pb-6" @click="menuOpen = false">
        <template v-for="(item, i) in nav" :key="i">
          <div v-if="'section' in item" class="px-2 pt-5 pb-1 text-[11px] font-medium tracking-wide text-zinc-400 uppercase">
            {{ item.section }}
          </div>
          <RouterLink
            v-else
            :to="linkTo(item.to!)"
            class="flex items-center justify-between rounded-md px-2 py-1.5 text-sm text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
            exact-active-class="!bg-zinc-100 !text-zinc-900 font-medium dark:!bg-zinc-800 dark:!text-zinc-100"
          >
            <span>{{ item.label }}</span>
            <span
              v-if="item.count !== undefined"
              class="tabular text-xs"
              :class="item.alert ? 'rounded bg-red-100 px-1.5 font-medium text-red-700 dark:bg-red-950 dark:text-red-300' : 'text-zinc-400'"
            >{{ item.count }}</span>
          </RouterLink>
        </template>
      </nav>
    </aside>
    <div v-if="menuOpen" class="fixed inset-0 z-20 bg-black/30 lg:hidden" @click="menuOpen = false" />

    <!-- Main -->
    <div class="min-w-0 flex-1">
      <header class="sticky top-0 z-10 flex h-14 items-center gap-3 border-b border-zinc-200 bg-zinc-50/85 px-4 backdrop-blur dark:border-zinc-800 dark:bg-zinc-950/85 sm:px-6">
        <button class="rounded p-1.5 text-zinc-500 hover:bg-zinc-200 lg:hidden dark:hover:bg-zinc-800" aria-label="Menu" @click="menuOpen = !menuOpen">
          <svg class="size-5" viewBox="0 0 20 20" fill="currentColor"><path d="M3 5h14v1.5H3zm0 4.25h14v1.5H3zm0 4.25h14V15H3z" /></svg>
        </button>
        <ProjectPicker :model-value="project" :projects="config?.projects ?? []" @update:model-value="selectProject" />
        <div class="ml-auto flex items-center gap-3 text-xs text-zinc-500">
          <span v-if="config?.projectRoot && config.projectRoot !== config.project" class="hidden font-mono md:inline">
            repo {{ tilde(config.projectRoot) }}
          </span>
          <button
            class="rounded-md border border-zinc-200 bg-white px-2.5 py-1 font-medium text-zinc-700 hover:bg-zinc-100 disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
            :disabled="loading"
            @click="refresh"
          >
            {{ loading ? 'Reading…' : 'Reload' }}
          </button>
        </div>
      </header>

      <main class="mx-auto max-w-7xl px-4 py-6 sm:px-6">
        <div v-if="error" class="mb-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-950 dark:text-red-200">
          {{ error }}
        </div>
        <div v-for="w in config?.warnings ?? []" :key="w" class="mb-4 rounded-md border border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200">
          {{ w }}
        </div>
        <RouterView v-if="config" />
        <FixDialog v-if="pendingFix" :key="JSON.stringify(pendingFix)" />
        <div v-else-if="loading" class="py-24 text-center text-sm text-zinc-500">Reading configuration…</div>
      </main>
    </div>
  </div>
</template>
