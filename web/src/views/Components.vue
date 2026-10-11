<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Component, Kind } from '../types'
import { relativeTime, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import SearchInput from '../components/SearchInput.vue'
import Segmented from '../components/Segmented.vue'
import ScopeBadge from '../components/ScopeBadge.vue'
import StatusDot from '../components/StatusDot.vue'
import Drawer from '../components/Drawer.vue'
import Field from '../components/Field.vue'
import Empty from '../components/Empty.vue'

const props = defineProps<{ kind: Kind }>()
const { config, findings } = useConfig()

const search = ref('')
const status = ref('active')
const origin = ref('all')
const selected = ref<Component | null>(null)
watch(() => props.kind, () => { search.value = ''; selected.value = null })

const meta: Record<Kind, { title: string; subtitle: string }> = {
  agent: {
    title: 'Agents',
    subtitle: 'Subagents Claude can delegate to. With the same name, a project agent beats a user agent, which beats a plugin agent.',
  },
  skill: {
    title: 'Skills',
    subtitle: 'Skills Claude loads when a request matches their description. With the same name, managed beats user beats project; plugin skills are namespaced.',
  },
  command: {
    title: 'Commands',
    subtitle: 'Files in commands/ directories. They share one namespace with skills, and a skill with the same name wins.',
  },
}

const all = computed(() => config.value?.components.filter((c) => c.kind === props.kind) ?? [])

const counts = computed(() => {
  const n = { active: 0, shadowed: 0, disabled: 0 }
  for (const c of all.value) n[c.status]++
  return n
})

const rows = computed(() => {
  const q = search.value.toLowerCase()
  return all.value
    .filter((c) => status.value === 'all' || c.status === status.value)
    .filter((c) => origin.value === 'all' || (origin.value === 'plugin' ? c.source.scope === 'plugin' : c.source.scope !== 'plugin'))
    .filter((c) => !q || c.name.toLowerCase().includes(q) || c.description.toLowerCase().includes(q) || (c.source.plugin ?? '').includes(q))
    .sort((a, b) => a.name.localeCompare(b.name))
})

const problemsByPath = computed(() => {
  const m = new Map<string, number>()
  for (const f of findings.value) if (f.path && f.severity !== 'info') m.set(f.path, (m.get(f.path) ?? 0) + 1)
  return m
})

const selectedFindings = computed(() => findings.value.filter((f) => selected.value && (f.path === selected.value.path || f.related === selected.value.path)))

const frontmatter = computed(() =>
  Object.entries(selected.value?.frontmatter ?? {}).filter(([k]) => k !== 'name' && k !== 'description'),
)

function show(v: unknown) {
  return typeof v === 'string' ? v : JSON.stringify(v)
}
</script>

<template>
  <PageHeader :title="meta[kind].title" :subtitle="meta[kind].subtitle">
    <Segmented
      v-model="status"
      :options="[
        { value: 'active', label: 'Active', count: counts.active },
        { value: 'shadowed', label: 'Shadowed', count: counts.shadowed },
        { value: 'disabled', label: 'Off', count: counts.disabled },
        { value: 'all', label: 'All', count: all.length },
      ]"
    />
    <Segmented v-model="origin" :options="[{ value: 'all', label: 'Any source' }, { value: 'own', label: 'Own' }, { value: 'plugin', label: 'Plugins' }]" />
    <SearchInput v-model="search" />
  </PageHeader>

  <Empty v-if="!rows.length">Nothing matches these filters.</Empty>
  <div v-else class="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
    <table class="w-full table-fixed text-sm">
      <thead class="border-b border-zinc-200 text-left text-xs text-zinc-500 dark:border-zinc-800">
        <tr>
          <th class="w-[34%] px-4 py-2 font-medium">Name</th>
          <th class="hidden px-4 py-2 font-medium md:table-cell">Description</th>
          <th class="w-32 px-4 py-2 font-medium">Source</th>
          <th v-if="kind !== 'agent'" class="hidden w-28 px-4 py-2 text-right font-medium sm:table-cell">Used</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-zinc-100 dark:divide-zinc-800">
        <tr
          v-for="c in rows"
          :key="c.path + c.name"
          class="cursor-pointer hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
          :class="c.status !== 'active' && 'opacity-60'"
          @click="selected = c"
        >
          <td class="px-4 py-2.5">
            <div class="flex items-center gap-2">
              <StatusDot :status="c.status" :note="c.statusNote" />
              <span class="truncate font-mono text-[13px]">{{ c.name }}</span>
              <span v-if="c.error || problemsByPath.get(c.path)" class="shrink-0 rounded bg-amber-100 px-1 text-[11px] font-medium text-amber-800 dark:bg-amber-950 dark:text-amber-300">!</span>
            </div>
          </td>
          <td class="hidden px-4 py-2.5 text-zinc-500 md:table-cell">
            <span class="line-clamp-1">{{ c.error ?? c.description }}</span>
          </td>
          <td class="px-4 py-2.5"><ScopeBadge :source="c.source" /></td>
          <td v-if="kind !== 'agent'" class="tabular hidden px-4 py-2.5 text-right text-zinc-500 sm:table-cell">
            <span v-if="c.usage" :title="relativeTime(c.usage.lastUsed)">{{ c.usage.count }}×</span>
            <span v-else class="text-zinc-300 dark:text-zinc-600">—</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <Drawer v-if="selected" :title="selected.name" :path="selected.error ? undefined : selected.path" @close="selected = null">
    <dl class="space-y-2">
      <Field label="Status">
        <span class="inline-flex items-center gap-2"><StatusDot :status="selected.status" />{{ selected.status }}</span>
        <div v-if="selected.statusNote" class="mt-0.5 text-xs text-zinc-500">{{ selected.statusNote }}</div>
      </Field>
      <Field label="Source"><ScopeBadge :source="selected.source" /></Field>
      <Field v-if="selected.description" label="Description">{{ selected.description }}</Field>
      <Field v-if="selected.usage" label="Used">{{ selected.usage.count }} times, last {{ relativeTime(selected.usage.lastUsed) }}</Field>
      <Field v-for="[k, v] in frontmatter" :key="k" :label="k"><span class="font-mono text-xs">{{ show(v) }}</span></Field>
    </dl>
    <div v-if="selected.error" class="rounded-md bg-red-50 px-3 py-2 text-red-800 dark:bg-red-950 dark:text-red-200">{{ selected.error }}</div>
    <ul v-if="selectedFindings.length" class="space-y-1.5">
      <li v-for="(f, i) in selectedFindings" :key="i" class="rounded-md bg-zinc-50 px-3 py-2 text-xs dark:bg-zinc-800">
        <span class="font-mono text-zinc-500">{{ f.rule }}</span> — {{ f.message }}
      </li>
    </ul>
  </Drawer>
</template>
