<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { MemoryEntry } from '../types'
import { tilde, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import SearchInput from '../components/SearchInput.vue'
import Segmented from '../components/Segmented.vue'
import Drawer from '../components/Drawer.vue'
import Field from '../components/Field.vue'
import Empty from '../components/Empty.vue'
import MemoryGraph from '../components/MemoryGraph.vue'

const { config, findings, project } = useConfig()
const storeDir = ref('')
const view = ref('list')
const search = ref('')
const selected = ref<MemoryEntry | null>(null)

const stores = computed(() =>
  [...(config.value?.memory ?? [])].sort((a, b) => b.entries.length - a.entries.length),
)
watch(stores, (s) => {
  if (!s.some((x) => x.dir === storeDir.value)) storeDir.value = s[0]?.dir ?? ''
}, { immediate: true })
const store = computed(() => stores.value.find((s) => s.dir === storeDir.value))

const indexed = computed(() => new Set(store.value?.index?.links.map((l) => l.file) ?? []))
const keys = computed(() => {
  const k = new Map<string, MemoryEntry>()
  for (const e of store.value?.entries ?? []) {
    k.set(e.name || e.file.replace(/\.md$/, ''), e)
    k.set(e.file.replace(/\.md$/, ''), e)
  }
  return k
})
const backlinks = computed(() => {
  const b = new Map<string, string[]>()
  for (const e of store.value?.entries ?? []) {
    for (const l of e.links) {
      const target = keys.value.get(l)
      if (target) b.set(target.path, [...(b.get(target.path) ?? []), e.name || e.file])
    }
  }
  return b
})

const rows = computed(() => {
  const q = search.value.toLowerCase()
  return (store.value?.entries ?? []).filter(
    (e) => !q || `${e.name} ${e.description} ${e.type}`.toLowerCase().includes(q),
  )
})

const types = computed(() => {
  const t = new Map<string, number>()
  for (const e of store.value?.entries ?? []) t.set(e.type || 'untyped', (t.get(e.type || 'untyped') ?? 0) + 1)
  return [...t.entries()].sort((a, b) => b[1] - a[1])
})

const storeFindings = computed(() => {
  const dir = store.value?.dir
  return dir ? findings.value.filter((f) => f.rule.startsWith('memory/') && f.path?.startsWith(dir + '/')) : []
})

const typeTone: Record<string, string> = {
  user: 'text-sky-700 bg-sky-50 dark:bg-sky-950 dark:text-sky-300',
  feedback: 'text-violet-700 bg-violet-50 dark:bg-violet-950 dark:text-violet-300',
  project: 'text-emerald-700 bg-emerald-50 dark:bg-emerald-950 dark:text-emerald-300',
  reference: 'text-amber-800 bg-amber-50 dark:bg-amber-950 dark:text-amber-300',
}

function open(path: string) {
  selected.value = store.value?.entries.find((e) => e.path === path) ?? null
}
</script>

<template>
  <PageHeader
    title="Memory"
    :subtitle="project
      ? 'The memory stores this project reads: its auto memory, plus any extra directories dotclaude was given.'
      : 'Every memory store on this machine. MEMORY.md is the index Claude Code loads; a memory missing from it is never recalled.'"
  >
    <Segmented v-model="view" :options="[{ value: 'list', label: 'List' }, { value: 'graph', label: 'Graph' }]" />
    <SearchInput v-model="search" />
  </PageHeader>

  <Empty v-if="!stores.length">No memory stores found.</Empty>
  <div v-else class="grid gap-6 lg:grid-cols-[16rem_1fr]">
    <nav class="space-y-0.5">
      <button
        v-for="s in stores"
        :key="s.dir"
        class="flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-left text-sm"
        :class="s.dir === storeDir ? 'bg-white font-medium shadow-sm ring-1 ring-zinc-200 dark:bg-zinc-900 dark:ring-zinc-800' : 'text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-900'"
        :title="s.dir"
        @click="storeDir = s.dir; selected = null"
      >
        <span class="truncate font-mono text-xs">{{ tilde(s.label).split('/').slice(-2).join('/') }}</span>
        <span class="tabular text-xs text-zinc-400">{{ s.entries.length }}</span>
      </button>
    </nav>

    <div v-if="store" class="min-w-0">
      <div class="mb-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-zinc-500">
        <span class="font-mono">{{ tilde(store.dir) }}</span>
        <span v-if="store.index">MEMORY.md: {{ store.index.lines }} lines, {{ store.index.links.length }} links</span>
        <span v-else class="text-amber-600">no MEMORY.md</span>
        <span v-for="[t, n] in types" :key="t">{{ n }} {{ t }}</span>
        <span v-if="storeFindings.length" class="text-amber-600">{{ storeFindings.length }} findings</span>
      </div>

      <MemoryGraph v-if="view === 'graph'" :entries="rows" @select="open" />

      <div v-else class="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
        <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
          <li v-for="e in rows" :key="e.path">
            <button class="w-full px-4 py-2.5 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50" @click="selected = e">
              <div class="flex items-center gap-2">
                <span class="truncate font-mono text-[13px]">{{ e.name || e.file }}</span>
                <span v-if="e.type" class="shrink-0 rounded px-1.5 text-[11px]" :class="typeTone[e.type] ?? 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800'">{{ e.type }}</span>
                <span v-if="store.index && !indexed.has(e.file)" class="shrink-0 rounded bg-amber-100 px-1.5 text-[11px] text-amber-800 dark:bg-amber-950 dark:text-amber-300">not indexed</span>
                <span v-if="e.links.length" class="ml-auto shrink-0 text-[11px] text-zinc-400">{{ e.links.length }} links</span>
              </div>
              <div class="mt-0.5 line-clamp-1 text-sm text-zinc-500">{{ e.description }}</div>
            </button>
          </li>
        </ul>
      </div>
    </div>
  </div>

  <Drawer v-if="selected" :title="selected.name || selected.file" :path="selected.path" @close="selected = null">
    <dl class="space-y-2">
      <Field label="Description">{{ selected.description || '—' }}</Field>
      <Field label="Type">{{ selected.type || '—' }}</Field>
      <Field label="In MEMORY.md">{{ !store?.index ? 'no index' : indexed.has(selected.file) ? 'yes' : 'no — never recalled' }}</Field>
      <Field v-if="selected.links.length" label="Links to">
        <span v-for="l in selected.links" :key="l" class="mr-2 inline-block">
          <button v-if="keys.get(l)" class="font-mono text-xs text-accent-600 hover:underline" @click="selected = keys.get(l)!">[[{{ l }}]]</button>
          <span v-else class="font-mono text-xs text-zinc-400" title="No memory with this name yet">[[{{ l }}]]</span>
        </span>
      </Field>
      <Field v-if="backlinks.get(selected.path)" label="Linked from">
        <span v-for="b in backlinks.get(selected.path)" :key="b" class="mr-2 inline-block font-mono text-xs">{{ b }}</span>
      </Field>
    </dl>
  </Drawer>
</template>
