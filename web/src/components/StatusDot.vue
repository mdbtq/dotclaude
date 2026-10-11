<script setup lang="ts">
import type { Status } from '../types'

defineProps<{ status: Status; note?: string }>()
</script>

<template>
  <span
    class="inline-block size-2 shrink-0 rounded-full"
    :class="{
      'bg-emerald-500': status === 'active',
      'bg-amber-400': status === 'shadowed',
      'bg-zinc-300 dark:bg-zinc-600': status === 'disabled',
    }"
    :title="note ? `${status}: ${note}` : status"
    role="img"
    :aria-label="status"
  />
</template>
