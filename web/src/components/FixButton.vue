<script setup lang="ts">
import { computed } from 'vue'
import type { FixRequest } from '../types'
import { canChange, requestFix, useConfig } from '../store'

const props = defineProps<{ fix: FixRequest | FixRequest[]; label?: string }>()
const fixes = computed(() => (Array.isArray(props.fix) ? props.fix : [props.fix]))
const { project } = useConfig()
</script>

<template>
  <button
    v-if="canChange"
    class="shrink-0 rounded-md border border-zinc-200 bg-white px-2 py-1 text-xs font-medium text-zinc-700 hover:border-accent-500 hover:text-accent-700 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:text-accent-500"
    @click.stop="requestFix(fixes, project)"
  >
    {{ label ?? fixes[0]?.label ?? 'Fix' }}
  </button>
</template>
