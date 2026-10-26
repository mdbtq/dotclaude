<script setup lang="ts">
import { computed, ref } from 'vue'
import { relativeTime, tilde, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import SearchInput from '../components/SearchInput.vue'
import Segmented from '../components/Segmented.vue'
import Empty from '../components/Empty.vue'
import FixButton from '../components/FixButton.vue'

const { config } = useConfig()
const search = ref('')
const filter = ref('enabled')

const all = computed(() => config.value?.plugins ?? [])
const rows = computed(() => {
  const q = search.value.toLowerCase()
  return all.value
    .filter((p) => filter.value === 'all' || (filter.value === 'enabled') === p.enabled)
    .filter((p) => !q || p.id.toLowerCase().includes(q) || (p.description ?? '').toLowerCase().includes(q))
})

const labels: Record<string, string> = { agents: 'agents', skills: 'skills', commands: 'commands', hooks: 'hooks', mcpServers: 'MCP' }
</script>

<template>
  <PageHeader title="Plugins" subtitle="Installed plugins and the plugins your settings enable, with what each one adds.">
    <Segmented
      v-model="filter"
      :options="[
        { value: 'enabled', label: 'Enabled', count: all.filter((p) => p.enabled).length },
        { value: 'disabled', label: 'Not enabled', count: all.filter((p) => !p.enabled).length },
        { value: 'all', label: 'All', count: all.length },
      ]"
    />
    <SearchInput v-model="search" />
  </PageHeader>

  <Empty v-if="!rows.length">No plugins match.</Empty>
  <div class="grid gap-3 md:grid-cols-2">
    <article
      v-for="p in rows"
      :key="p.id"
      class="rounded-lg border bg-white px-4 py-3 dark:bg-zinc-900"
      :class="p.error && p.enabled ? 'border-red-300 dark:border-red-900' : 'border-zinc-200 dark:border-zinc-800'"
    >
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <h3 class="truncate font-mono text-[13px] font-semibold" :class="!p.enabled && 'text-zinc-500'">{{ p.name }}</h3>
          <div class="truncate text-xs text-zinc-400">{{ p.marketplace }}<template v-if="p.version"> · {{ p.version }}</template><template v-if="p.author"> · {{ p.author }}</template></div>
        </div>
        <span
          class="shrink-0 rounded px-1.5 py-0.5 text-[11px] font-medium"
          :class="p.enabled ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' : 'bg-zinc-100 text-zinc-500 dark:bg-zinc-800'"
          :title="p.enabledBy ? `decided by ${p.enabledBy} settings` : 'not mentioned in enabledPlugins'"
        >{{ p.enabled ? 'enabled' : 'off' }}<template v-if="p.enabledBy"> · {{ p.enabledBy }}</template></span>
      </div>
      <p v-if="p.description" class="mt-2 line-clamp-2 text-sm text-zinc-600 dark:text-zinc-400">{{ p.description }}</p>
      <p v-if="p.error" class="mt-2 text-sm text-red-600">{{ p.error }}</p>
      <div class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-500">
        <template v-for="(label, key) in labels" :key="key">
          <span v-if="p.counts[key]"><b class="tabular font-medium text-zinc-700 dark:text-zinc-300">{{ p.counts[key] }}</b> {{ label }}</span>
        </template>
        <span v-if="p.usage" class="ml-auto">used {{ p.usage.count }}× · {{ relativeTime(p.usage.lastUsed) }}</span>
      </div>
      <div class="mt-2 flex items-center justify-between gap-3">
        <span class="truncate font-mono text-[11px] text-zinc-400" :title="p.installPath">{{ tilde(p.installPath) }}</span>
        <FixButton
          v-if="p.enabledBy !== 'managed' && (p.enabled || p.installed)"
          :fix="{ kind: 'plugin-enabled', target: p.id, value: p.enabled ? 'false' : 'true', label: p.enabled ? 'Disable' : 'Enable' }"
        />
      </div>
    </article>
  </div>
</template>
