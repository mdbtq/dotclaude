<script setup lang="ts">
defineProps<{ label: string }>()
</script>

<template>
  <div class="grid grid-cols-[8rem_1fr] gap-3">
    <dt class="text-zinc-500">{{ label }}</dt>
    <dd class="min-w-0 break-words"><slot /></dd>
  </div>
</template>
