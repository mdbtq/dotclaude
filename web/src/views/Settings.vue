<script setup lang="ts">
import { computed, ref } from 'vue'
import { tilde, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import SearchInput from '../components/SearchInput.vue'
import Segmented from '../components/Segmented.vue'
import ScopeBadge from '../components/ScopeBadge.vue'

const { config } = useConfig()
const search = ref('')
const only = ref('all')

const rows = computed(() => {
  const q = search.value.toLowerCase()
  return (config.value?.settings ?? [])
    .filter((s) => only.value === 'all' || (s.overridden?.length ?? 0) > 0)
    .filter((s) => !q || s.key.toLowerCase().includes(q) || format(s.value).toLowerCase().includes(q))
})

function format(v: unknown): string {
  if (typeof v === 'string') return v
  return JSON.stringify(v, null, Array.isArray(v) && v.length > 3 ? 1 : undefined) ?? ''
}
</script>

<template>
  <PageHeader
    title="Settings"
    subtitle="Settings files merged in precedence order. Lists combine across files; any other value comes from the highest file that sets it. Values that look like secrets, and everything under env, are redacted."
  >
    <Segmented v-model="only" :options="[{ value: 'all', label: 'All keys' }, { value: 'multi', label: 'Set in several files' }]" />
    <SearchInput v-model="search" />
  </PageHeader>

  <h2 class="mb-2 text-sm font-semibold">Files, highest precedence first</h2>
  <ol class="mb-6 divide-y divide-zinc-100 rounded-lg border border-zinc-200 bg-white text-sm dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900">
    <li v-for="(l, i) in config!.settingsLayers" :key="l.path" class="px-4 py-2" :class="(!l.exists || l.ignored) && 'opacity-50'">
      <div class="flex items-center gap-3">
        <span class="tabular w-4 text-xs text-zinc-400">{{ l.ignored ? '–' : i + 1 }}</span>
        <span class="w-20"><ScopeBadge :source="{ scope: l.scope }" /></span>
        <span class="min-w-0 flex-1 truncate font-mono text-[13px]" :class="l.ignored && 'line-through'">{{ tilde(l.path) }}</span>
        <span class="text-xs" :class="l.error ? 'text-red-600' : 'text-zinc-400'">{{ l.error ?? (l.ignored ? 'not read here' : l.exists ? '' : 'not present') }}</span>
      </div>
      <p v-if="l.ignored" class="mt-0.5 ml-[7.75rem] text-xs text-zinc-500">{{ l.ignored }}</p>
    </li>
  </ol>

  <div class="overflow-x-auto rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
    <table class="w-full text-sm">
      <thead class="border-b border-zinc-200 text-left text-xs text-zinc-500 dark:border-zinc-800">
        <tr>
          <th class="px-4 py-2 font-medium">Key</th>
          <th class="px-4 py-2 font-medium">Effective value</th>
          <th class="px-4 py-2 font-medium">From</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-zinc-100 align-top dark:divide-zinc-800">
        <tr v-for="s in rows" :key="s.key">
          <td class="px-4 py-2 font-mono text-[13px] whitespace-nowrap">{{ s.key }}</td>
          <td class="max-w-xl px-4 py-2"><pre class="font-mono text-xs whitespace-pre-wrap break-all text-zinc-600 dark:text-zinc-400">{{ format(s.value) }}</pre></td>
          <td class="px-4 py-2 whitespace-nowrap">
            <ScopeBadge :source="{ scope: s.scope }" />
            <div v-if="s.overridden?.length" class="mt-1 text-[11px] text-zinc-400">
              {{ s.merged ? 'merged with' : 'overrides' }} {{ s.overridden.join(', ') }}
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
