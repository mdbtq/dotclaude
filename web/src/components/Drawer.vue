<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { fetchFile, tilde, useConfig } from '../store'

const props = defineProps<{ title: string; path?: string }>()
const emit = defineEmits<{ close: [] }>()

const { project } = useConfig()
const content = ref<string | null>(null)
const failed = ref<string | null>(null)

watch(
  () => props.path,
  async (path) => {
    content.value = null
    failed.value = null
    if (!path) return
    try {
      content.value = await fetchFile(path, project.value)
    } catch (e) {
      failed.value = e instanceof Error ? e.message : String(e)
    }
  },
  { immediate: true },
)

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

async function copyPath() {
  if (props.path) await navigator.clipboard?.writeText(props.path)
}
</script>

<template>
  <div class="fixed inset-0 z-40 bg-black/20 dark:bg-black/50" @click="emit('close')" />
  <aside
    class="fixed inset-y-0 right-0 z-50 flex w-full max-w-2xl flex-col border-l border-zinc-200 bg-white shadow-xl dark:border-zinc-800 dark:bg-zinc-900"
    role="dialog"
    :aria-label="title"
  >
    <div class="flex items-start justify-between gap-4 border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
      <div class="min-w-0">
        <h2 class="truncate font-semibold">{{ title }}</h2>
        <button
          v-if="path"
          class="mt-0.5 max-w-full truncate text-left font-mono text-xs text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200"
          title="Copy path"
          @click="copyPath"
        >{{ tilde(path) }}</button>
      </div>
      <button class="rounded p-1 text-zinc-400 hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-800" aria-label="Close" @click="emit('close')">
        <svg class="size-5" viewBox="0 0 20 20" fill="currentColor"><path d="M5.3 4.2L10 8.9l4.7-4.7 1.1 1.1-4.7 4.7 4.7 4.7-1.1 1.1-4.7-4.7-4.7 4.7-1.1-1.1 4.7-4.7-4.7-4.7z" /></svg>
      </button>
    </div>
    <div class="flex-1 overflow-y-auto">
      <div class="space-y-4 px-5 py-4 text-sm empty:hidden"><slot /></div>
      <div v-if="path" class="border-t border-zinc-200 dark:border-zinc-800">
        <div v-if="failed" class="px-5 py-4 text-sm text-red-600">{{ failed }}</div>
        <div v-else-if="content === null" class="px-5 py-4 text-sm text-zinc-400">Reading file…</div>
        <pre v-else class="px-5 py-4 font-mono text-[12.5px] leading-relaxed whitespace-pre-wrap break-words text-zinc-700 dark:text-zinc-300">{{ content }}</pre>
      </div>
    </div>
  </aside>
</template>
