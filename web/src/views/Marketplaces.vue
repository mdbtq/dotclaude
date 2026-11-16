<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Marketplace } from '../types'
import { relativeTime, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import SearchInput from '../components/SearchInput.vue'
import Empty from '../components/Empty.vue'

const { project, config } = useConfig()
const search = ref('')

const rows = computed(() => {
  const q = search.value.toLowerCase()
  return (config.value?.marketplaces ?? [])
    .filter((m) => !q || m.name.toLowerCase().includes(q) || (m.description ?? '').toLowerCase().includes(q) || (m.location ?? '').toLowerCase().includes(q))
    .map((m) => ({ m, s: summary(m) }))
})

// What each marketplace contributes here: its plugins and their components.
function summary(m: Marketplace) {
  const plugins = config.value?.plugins?.filter((p) => p.marketplace === m.name) ?? []
  const kinds = { agent: 0, skill: 0, command: 0 }
  for (const c of config.value?.components ?? []) {
    if (c.status === 'active' && c.source.plugin?.endsWith('@' + m.name)) kinds[c.kind]++
  }
  return { plugins: plugins.length, enabled: plugins.filter((p) => p.enabled).length, kinds }
}

function linkTo(name: string) {
  return { path: `/marketplaces/${encodeURIComponent(name)}`, query: project.value ? { project: project.value } : {} }
}
</script>

<template>
  <PageHeader title="Marketplaces" subtitle="The plugin marketplaces Claude Code knows, and what the plugins you installed from each one add.">
    <SearchInput v-model="search" />
  </PageHeader>

  <Empty v-if="!rows.length">No marketplaces match.</Empty>
  <div class="grid gap-3 md:grid-cols-2">
    <RouterLink
      v-for="{ m, s } in rows"
      :key="m.name"
      :to="linkTo(m.name)"
      class="block rounded-lg border bg-white px-4 py-3 hover:border-zinc-300 dark:bg-zinc-900 dark:hover:border-zinc-700"
      :class="m.error ? 'border-red-300 dark:border-red-900' : 'border-zinc-200 dark:border-zinc-800'"
    >
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <h3 class="truncate font-mono text-[13px] font-semibold">{{ m.name }}</h3>
          <div class="truncate text-xs text-zinc-400">
            <template v-if="m.location">{{ m.source }} · {{ m.location }}</template>
            <template v-if="m.owner"> · {{ m.owner }}</template>
          </div>
        </div>
        <span
          class="tabular shrink-0 rounded px-1.5 py-0.5 text-[11px] font-medium"
          :class="s.enabled ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' : 'bg-zinc-100 text-zinc-500 dark:bg-zinc-800'"
          :title="`${s.enabled} of ${s.plugins} plugins from here enabled; the marketplace lists ${m.listed}`"
        >{{ s.enabled }}/{{ s.plugins }} enabled</span>
      </div>
      <p v-if="m.description" class="mt-2 line-clamp-2 text-sm text-zinc-600 dark:text-zinc-400">{{ m.description }}</p>
      <p v-if="m.error" class="mt-2 text-sm text-red-600">{{ m.error }}</p>
      <div class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-500">
        <span v-if="s.kinds.agent"><b class="tabular font-medium text-zinc-700 dark:text-zinc-300">{{ s.kinds.agent }}</b> agents</span>
        <span v-if="s.kinds.skill"><b class="tabular font-medium text-zinc-700 dark:text-zinc-300">{{ s.kinds.skill }}</b> skills</span>
        <span v-if="s.kinds.command"><b class="tabular font-medium text-zinc-700 dark:text-zinc-300">{{ s.kinds.command }}</b> commands</span>
        <span v-if="m.known"><b class="tabular font-medium text-zinc-700 dark:text-zinc-300">{{ m.listed }}</b> listed</span>
        <span v-if="m.lastUpdated" class="ml-auto">updated {{ relativeTime(m.lastUpdated) }}</span>
      </div>
    </RouterLink>
  </div>
</template>
