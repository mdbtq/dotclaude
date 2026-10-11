<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Hook } from '../types'
import { tilde, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import SearchInput from '../components/SearchInput.vue'
import ScopeBadge from '../components/ScopeBadge.vue'
import StatusDot from '../components/StatusDot.vue'
import Empty from '../components/Empty.vue'

const { config } = useConfig()
const search = ref('')

const groups = computed(() => {
  const q = search.value.toLowerCase()
  const by = new Map<string, Hook[]>()
  for (const h of config.value?.hooks ?? []) {
    if (q && !`${h.event} ${h.matcher ?? ''} ${h.detail} ${h.source.plugin ?? ''}`.toLowerCase().includes(q)) continue
    by.set(h.event, [...(by.get(h.event) ?? []), h])
  }
  return [...by.entries()].sort(([a], [b]) => a.localeCompare(b))
})
</script>

<template>
  <PageHeader title="Hooks" subtitle="Every handler that runs on a hook event. Hooks from all settings files and enabled plugins run side by side; none overrides another.">
    <SearchInput v-model="search" />
  </PageHeader>

  <Empty v-if="!groups.length">No hooks.</Empty>
  <section v-for="[event, hooks] in groups" :key="event" class="mb-5">
    <h2 class="mb-2 flex items-baseline gap-2 text-sm font-semibold">
      {{ event }} <span class="tabular text-xs font-normal text-zinc-400">{{ hooks.length }}</span>
    </h2>
    <div class="divide-y divide-zinc-100 overflow-hidden rounded-lg border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900">
      <div v-for="(h, i) in hooks" :key="i" class="grid gap-x-4 gap-y-1 px-4 py-2.5 text-sm md:grid-cols-[10rem_1fr_8rem]" :class="h.status !== 'active' && 'opacity-60'">
        <div class="flex items-center gap-2">
          <StatusDot :status="h.status" />
          <span class="truncate font-mono text-xs text-zinc-500" :title="h.matcher">{{ h.matcher || '*' }}</span>
        </div>
        <div class="min-w-0">
          <code class="block font-mono text-[12.5px] break-all">{{ h.detail }}</code>
          <div class="mt-0.5 truncate font-mono text-[11px] text-zinc-400">{{ h.type }}<template v-if="h.timeout"> · {{ h.timeout }}s</template> · {{ tilde(h.path) }}</div>
        </div>
        <div class="md:text-right"><ScopeBadge :source="h.source" /></div>
      </div>
    </div>
  </section>
</template>
