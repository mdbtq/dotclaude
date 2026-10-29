<script setup lang="ts">
import type { Instruction } from '../types'
import { tilde } from '../store'
import ScopeBadge from './ScopeBadge.vue'

defineProps<{ item: Instruction; index?: number; depth: number }>()
const emit = defineEmits<{ select: [item: Instruction] }>()

const kinds: Record<string, string> = {
  'claude-md': 'CLAUDE.md',
  local: 'local',
  'agents-md': 'AGENTS.md',
  rule: 'rule',
  import: 'import',
}
</script>

<template>
  <li class="border-b border-zinc-100 last:border-b-0 dark:border-zinc-800">
    <button
      class="flex w-full items-center gap-3 py-2.5 pr-4 text-left text-sm hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
      :style="{ paddingLeft: `${1 + depth * 1.5}rem` }"
      @click="emit('select', item)"
    >
      <span v-if="index" class="tabular w-5 shrink-0 text-right text-xs text-zinc-400">{{ index }}</span>
      <span v-else class="w-5 shrink-0 text-right text-zinc-300 dark:text-zinc-600">↳</span>
      <span class="min-w-0 flex-1">
        <span class="block truncate font-mono text-[13px]" :class="!item.exists && 'text-red-600 line-through'">
          {{ item.import ?? tilde(item.path) }}
        </span>
        <span v-if="item.import" class="block truncate font-mono text-[11px] text-zinc-400">line {{ item.line }} → {{ tilde(item.path) }}</span>
        <span v-if="item.appliesUnder" class="block truncate text-[11px] text-zinc-400">
          projects under {{ tilde(item.appliesUnder) }} · {{ item.projects }} known
        </span>
      </span>
      <span v-if="item.conditional" class="shrink-0 rounded bg-zinc-100 px-1.5 py-0.5 text-[11px] text-zinc-500 dark:bg-zinc-800">path-scoped</span>
      <span v-if="item.external" class="shrink-0 rounded bg-amber-50 px-1.5 py-0.5 text-[11px] text-amber-700 dark:bg-amber-950 dark:text-amber-300">external</span>
      <span class="hidden w-20 shrink-0 text-xs text-zinc-400 sm:block">{{ kinds[item.kind] ?? item.kind }}</span>
      <span class="w-16 shrink-0"><ScopeBadge v-if="!depth" :source="{ scope: item.scope }" /></span>
      <span class="tabular w-20 shrink-0 text-right text-xs" :class="item.lines > 200 ? 'text-amber-600' : 'text-zinc-400'">
        {{ item.exists ? `${item.lines} lines` : 'missing' }}
      </span>
    </button>
    <ol v-if="item.imports?.length">
      <InstructionRow v-for="c in item.imports" :key="c.path + c.line" :item="c" :depth="depth + 1" @select="emit('select', $event)" />
    </ol>
  </li>
</template>
