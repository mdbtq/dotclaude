<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import type { Project } from '../types'
import { tilde } from '../store'

const props = defineProps<{ modelValue: string; projects: Project[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const open = ref(false)
const search = ref('')
const input = ref<HTMLInputElement | null>(null)
const active = ref(0)

const matches = computed(() => {
  const q = search.value.toLowerCase()
  return props.projects
    .filter((p) => p.exists && !p.path.includes('/scratchpad'))
    .filter((p) => !q || p.path.toLowerCase().includes(q))
    .slice(0, 50)
})

async function toggle() {
  open.value = !open.value
  if (open.value) {
    search.value = ''
    active.value = 0
    await nextTick()
    input.value?.focus()
  }
}

function choose(path: string) {
  open.value = false
  emit('update:modelValue', path)
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') active.value = Math.min(active.value + 1, matches.value.length)
  else if (e.key === 'ArrowUp') active.value = Math.max(active.value - 1, 0)
  else if (e.key === 'Escape') open.value = false
  else if (e.key === 'Enter') {
    if (active.value === 0) choose('')
    else if (matches.value[active.value - 1]) choose(matches.value[active.value - 1].path)
    else if (search.value.startsWith('/') || search.value.startsWith('~')) choose(search.value)
    return
  } else return
  e.preventDefault()
}
</script>

<template>
  <div class="relative min-w-0">
    <button
      class="flex max-w-full items-center gap-2 rounded-md border border-zinc-200 bg-white px-3 py-1.5 text-left text-sm hover:border-zinc-300 dark:border-zinc-700 dark:bg-zinc-900 dark:hover:border-zinc-600"
      @click="toggle"
    >
      <span class="text-xs text-zinc-400">{{ modelValue ? 'Project' : 'View' }}</span>
      <span class="truncate font-medium" :class="modelValue && 'font-mono text-[13px]'">
        {{ modelValue ? tilde(modelValue) : 'User level — every project' }}
      </span>
      <svg class="size-4 shrink-0 text-zinc-400" viewBox="0 0 20 20" fill="currentColor"><path d="M5.5 8l4.5 4.5L14.5 8z" /></svg>
    </button>

    <div v-if="open" class="fixed inset-0 z-40" @click="open = false" />
    <div
      v-if="open"
      class="absolute left-0 z-50 mt-1.5 w-[min(36rem,calc(100vw-2rem))] overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-lg dark:border-zinc-700 dark:bg-zinc-900"
    >
      <input
        ref="input"
        v-model="search"
        class="w-full border-b border-zinc-200 bg-transparent px-3 py-2.5 text-sm outline-none placeholder:text-zinc-400 dark:border-zinc-700"
        placeholder="Filter projects, or type a path and press Enter"
        @keydown="onKey"
        @input="active = 0"
      />
      <ul class="max-h-96 overflow-y-auto py-1 text-sm">
        <li>
          <button class="w-full px-3 py-1.5 text-left" :class="active === 0 && 'bg-zinc-100 dark:bg-zinc-800'" @mouseenter="active = 0" @click="choose('')">
            User level <span class="text-zinc-400">— what applies in every project</span>
          </button>
        </li>
        <li v-for="(p, i) in matches" :key="p.path">
          <button
            class="flex w-full items-center justify-between gap-3 px-3 py-1.5 text-left font-mono text-[13px]"
            :class="active === i + 1 && 'bg-zinc-100 dark:bg-zinc-800'"
            @mouseenter="active = i + 1"
            @click="choose(p.path)"
          >
            <span class="truncate">{{ tilde(p.path) }}</span>
            <span v-if="p.hasMemory" class="shrink-0 font-sans text-[11px] text-zinc-400">memory</span>
          </button>
        </li>
        <li v-if="!matches.length" class="px-3 py-2 text-zinc-400">No known project matches.</li>
      </ul>
    </div>
  </div>
</template>
