<script setup lang="ts">
defineProps<{ options: { value: string; label: string; count?: number }[] }>()
const model = defineModel<string>({ required: true })
</script>

<template>
  <div class="inline-flex rounded-md border border-zinc-200 bg-white p-0.5 text-xs dark:border-zinc-700 dark:bg-zinc-900">
    <button
      v-for="o in options"
      :key="o.value"
      class="rounded px-2.5 py-1 font-medium"
      :class="model === o.value ? 'bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-900' : 'text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100'"
      @click="model = o.value"
    >
      {{ o.label }}<span v-if="o.count !== undefined" class="tabular ml-1 opacity-60">{{ o.count }}</span>
    </button>
  </div>
</template>
