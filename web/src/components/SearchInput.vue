<script setup lang="ts">
defineProps<{ placeholder?: string }>()
const model = defineModel<string>({ required: true })
</script>

<template>
  <input
    v-model="model"
    type="search"
    :placeholder="placeholder ?? 'Filter'"
    class="w-56 rounded-md border border-zinc-200 bg-white px-3 py-1.5 text-sm outline-none placeholder:text-zinc-400 focus:border-accent-500 dark:border-zinc-700 dark:bg-zinc-900"
  />
</template>
