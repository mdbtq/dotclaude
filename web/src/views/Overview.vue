<script setup lang="ts">
import { computed } from 'vue'
import type { Instruction } from '../types'
import { relativeTime, tilde, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import ScopeBadge from '../components/ScopeBadge.vue'

const { config, findings, project } = useConfig()

function linkTo(path: string) {
  return { path, query: project.value ? { project: project.value } : {} }
}

const stats = computed(() => {
  const c = config.value!
  const comps = (kind: string) => c.components.filter((x) => x.kind === kind)
  const active = (list: { status: string }[]) => list.filter((x) => x.status === 'active').length
  return [
    { label: 'Agents', to: '/agents', value: active(comps('agent')), total: comps('agent').length },
    { label: 'Skills', to: '/skills', value: active(comps('skill')), total: comps('skill').length },
    { label: 'Commands', to: '/commands', value: active(comps('command')), total: comps('command').length },
    { label: 'Plugins enabled', to: '/plugins', value: c.plugins?.filter((p) => p.enabled).length ?? 0, total: c.plugins?.length ?? 0 },
    { label: 'Hooks', to: '/hooks', value: active(c.hooks ?? []), total: c.hooks?.length ?? 0 },
    { label: 'MCP servers', to: '/mcp', value: active(c.mcpServers ?? []), total: c.mcpServers?.length ?? 0 },
    { label: 'Memories', to: '/memory', value: c.memory?.reduce((n, s) => n + s.entries.length, 0) ?? 0, total: undefined, note: `${c.memory?.length ?? 0} stores` },
    { label: 'Instruction files', to: '/instructions', value: c.instructions.length, total: undefined },
  ]
})

const severity = computed(() => ({
  error: findings.value.filter((f) => f.severity === 'error').length,
  warning: findings.value.filter((f) => f.severity === 'warning').length,
  info: findings.value.filter((f) => f.severity === 'info').length,
}))

const topProblems = computed(() => findings.value.filter((f) => f.severity !== 'info').slice(0, 6))

const mostUsed = computed(() =>
  (config.value?.components ?? [])
    .filter((c) => c.usage && c.status === 'active')
    .sort((a, b) => b.usage!.count - a.usage!.count)
    .slice(0, 8),
)

const instructionLines = computed(() => {
  let n = 0
  const walk = (list: Instruction[]) => {
    for (const i of list) {
      if (!i.conditional) n += i.lines
      walk(i.imports ?? [])
    }
  }
  walk(config.value?.instructions ?? [])
  return n
})
</script>

<template>
  <PageHeader
    :title="project ? 'In this project' : 'User level'"
    :subtitle="project
      ? `What Claude Code loads when started in ${tilde(project)}: your user configuration plus everything this project adds.`
      : 'What applies in every project. Pick a project at the top to see what it adds and overrides.'"
  />

  <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
    <RouterLink
      v-for="s in stats"
      :key="s.label"
      :to="linkTo(s.to)"
      class="rounded-lg border border-zinc-200 bg-white px-4 py-3 hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-700"
    >
      <div class="text-xs text-zinc-500">{{ s.label }}</div>
      <div class="mt-1 flex items-baseline gap-1.5">
        <span class="tabular text-2xl font-semibold tracking-tight">{{ s.value }}</span>
        <span v-if="s.total !== undefined && s.total !== s.value" class="tabular text-xs text-zinc-400">of {{ s.total }}</span>
        <span v-if="s.note" class="text-xs text-zinc-400">in {{ s.note }}</span>
      </div>
    </RouterLink>
  </div>

  <div class="mt-6 grid gap-6 lg:grid-cols-2">
    <section class="rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
        <h2 class="text-sm font-semibold">Health</h2>
        <RouterLink :to="linkTo('/lint')" class="text-xs text-accent-600 hover:underline">All findings →</RouterLink>
      </div>
      <div class="flex gap-6 px-4 pt-3 text-sm">
        <span><b class="tabular text-red-600">{{ severity.error }}</b> errors</span>
        <span><b class="tabular text-amber-600">{{ severity.warning }}</b> warnings</span>
        <span><b class="tabular text-zinc-500">{{ severity.info }}</b> info</span>
      </div>
      <ul class="divide-y divide-zinc-100 px-4 py-2 text-sm dark:divide-zinc-800">
        <li v-for="(f, i) in topProblems" :key="i" class="py-2">
          <div class="flex items-center gap-2">
            <span class="size-1.5 shrink-0 rounded-full" :class="f.severity === 'error' ? 'bg-red-500' : 'bg-amber-400'" />
            <span class="truncate">{{ f.message }}</span>
          </div>
          <div v-if="f.path" class="ml-3.5 truncate font-mono text-xs text-zinc-400">{{ tilde(f.path) }}</div>
        </li>
        <li v-if="!topProblems.length" class="py-3 text-zinc-500">No errors or warnings.</li>
      </ul>
    </section>

    <section class="rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
        <h2 class="text-sm font-semibold">Instructions loaded at launch</h2>
        <RouterLink :to="linkTo('/instructions')" class="text-xs text-accent-600 hover:underline">Details →</RouterLink>
      </div>
      <ol class="divide-y divide-zinc-100 px-4 py-2 text-sm dark:divide-zinc-800">
        <li v-for="i in config!.instructions" :key="i.path" class="flex items-center justify-between gap-3 py-2">
          <span class="flex min-w-0 items-center gap-2">
            <ScopeBadge :source="{ scope: i.scope }" />
            <span class="truncate font-mono text-xs">{{ tilde(i.path) }}</span>
          </span>
          <span class="tabular shrink-0 text-xs text-zinc-400">{{ i.lines }} lines</span>
        </li>
      </ol>
      <div class="border-t border-zinc-100 px-4 py-2 text-xs text-zinc-500 dark:border-zinc-800">
        {{ instructionLines }} lines in total, imports included and path-scoped rules left out.
      </div>
    </section>

    <section v-if="mostUsed.length" class="rounded-lg border border-zinc-200 bg-white lg:col-span-2 dark:border-zinc-800 dark:bg-zinc-900">
      <div class="border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
        <h2 class="text-sm font-semibold">Most used skills and commands</h2>
      </div>
      <ul class="grid gap-x-6 px-4 py-2 text-sm sm:grid-cols-2">
        <li v-for="c in mostUsed" :key="c.path" class="flex items-center justify-between gap-3 py-1.5">
          <span class="truncate font-mono text-[13px]">{{ c.name }}</span>
          <span class="tabular shrink-0 text-xs text-zinc-400">{{ c.usage!.count }}× · {{ relativeTime(c.usage!.lastUsed) }}</span>
        </li>
      </ul>
    </section>
  </div>
</template>
