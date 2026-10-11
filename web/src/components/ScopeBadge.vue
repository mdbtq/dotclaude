<script setup lang="ts">
import type { Source } from '../types'

defineProps<{ source: Source }>()

const tone: Record<string, string> = {
  managed: 'bg-purple-50 text-purple-700 ring-purple-200 dark:bg-purple-950 dark:text-purple-300 dark:ring-purple-900',
  user: 'bg-sky-50 text-sky-700 ring-sky-200 dark:bg-sky-950 dark:text-sky-300 dark:ring-sky-900',
  project: 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:ring-emerald-900',
  local: 'bg-amber-50 text-amber-800 ring-amber-200 dark:bg-amber-950 dark:text-amber-300 dark:ring-amber-900',
  plugin: 'bg-zinc-100 text-zinc-600 ring-zinc-200 dark:bg-zinc-800 dark:text-zinc-300 dark:ring-zinc-700',
}
</script>

<template>
  <span
    class="inline-flex max-w-full items-center truncate rounded px-1.5 py-0.5 text-[11px] font-medium ring-1 ring-inset"
    :class="tone[source.scope]"
    :title="source.plugin ?? source.scope"
  >{{ source.plugin ? source.plugin.split('@')[0] : source.scope }}</span>
</template>
