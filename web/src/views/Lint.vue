<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Finding, Severity } from '../types'
import { tilde, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import Segmented from '../components/Segmented.vue'
import SearchInput from '../components/SearchInput.vue'
import Drawer from '../components/Drawer.vue'
import Empty from '../components/Empty.vue'
import FixButton from '../components/FixButton.vue'

const { findings, meta } = useConfig()
const level = ref<'warning' | 'error' | 'info'>('warning')
const search = ref('')
const open = ref<string | null>(null)

const rank: Record<Severity, number> = { error: 0, warning: 1, info: 2 }

const groups = computed(() => {
  const q = search.value.toLowerCase()
  const by = new Map<string, Finding[]>()
  for (const f of findings.value) {
    if (rank[f.severity] > rank[level.value]) continue
    if (q && !`${f.rule} ${f.message} ${f.path ?? ''}`.toLowerCase().includes(q)) continue
    by.set(f.rule, [...(by.get(f.rule) ?? []), f])
  }
  return [...by.entries()]
})

const describe = (rule: string) => meta.value?.rules.find((r) => r.id === rule)?.description
const count = (s: Severity) => findings.value.filter((f) => f.severity === s).length
const fixesOf = (list: Finding[]) => list.flatMap((f) => (f.fix ? [f.fix] : []))
</script>

<template>
  <PageHeader title="Lint" subtitle="Problems in the configuration shown. Run the same checks from a terminal or CI with `dotclaude lint`.">
    <Segmented
      v-model="level"
      :options="[
        { value: 'error', label: 'Errors', count: count('error') },
        { value: 'warning', label: '+ Warnings', count: count('warning') },
        { value: 'info', label: '+ Info', count: count('info') },
      ]"
    />
    <SearchInput v-model="search" />
  </PageHeader>

  <Empty v-if="!groups.length">Nothing to report at this level.</Empty>
  <section v-for="[rule, list] in groups" :key="rule" class="mb-5">
    <h2 class="flex items-center gap-2 text-sm font-semibold">
      <span
        class="size-2 rounded-full"
        :class="{ 'bg-red-500': list[0].severity === 'error', 'bg-amber-400': list[0].severity === 'warning', 'bg-zinc-400': list[0].severity === 'info' }"
      />
      <span class="font-mono">{{ rule }}</span>
      <span class="tabular text-xs font-normal text-zinc-400">{{ list.length }}</span>
      <span v-if="fixesOf(list).length > 1" class="ml-auto font-normal">
        <FixButton :fix="fixesOf(list)" :label="`${fixesOf(list)[0].label} — all ${fixesOf(list).length}`" />
      </span>
    </h2>
    <p v-if="describe(rule)" class="mt-0.5 mb-2 ml-4 text-xs text-zinc-500">{{ describe(rule) }}</p>
    <ul class="divide-y divide-zinc-100 overflow-hidden rounded-lg border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900">
      <li v-for="(f, i) in list" :key="i" class="flex items-center gap-3 pr-4">
        <button class="min-w-0 flex-1 px-4 py-2 text-left text-sm hover:bg-zinc-50 disabled:cursor-default dark:hover:bg-zinc-800/50" :disabled="!f.path?.endsWith('.md')" @click="open = f.path ?? null">
          <div>{{ f.message }}</div>
          <div v-if="f.path" class="truncate font-mono text-[11px] text-zinc-400">
            {{ tilde(f.path) }}<template v-if="f.line">:{{ f.line }}</template>
            <template v-if="f.related"> ↔ {{ tilde(f.related) }}</template>
          </div>
        </button>
        <FixButton v-if="f.fix" :fix="f.fix" />
      </li>
    </ul>
  </section>

  <Drawer v-if="open" :title="tilde(open).split('/').slice(-2).join('/')" :path="open" @close="open = null" />
</template>
