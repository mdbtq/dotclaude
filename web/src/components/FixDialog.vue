<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import type { FixPlan, FixResult } from '../types'
import { applyFix, pendingFix, previewFix, tilde } from '../store'

const plan = ref<FixPlan | null>(null)
const result = ref<FixResult | null>(null)
const error = ref<string | null>(null)
const busy = ref(false)

const { requests, project } = pendingFix.value!

previewFix(requests, project)
  .then((p) => (plan.value = p))
  .catch((e) => (error.value = e instanceof Error ? e.message : String(e)))

async function apply() {
  if (!plan.value) return
  busy.value = true
  error.value = null
  try {
    result.value = await applyFix(plan.value, project)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}

function close() {
  pendingFix.value = null
}

function lineClass(line: string) {
  if (line.startsWith('@@')) return 'text-zinc-400'
  if (line.startsWith('+')) return 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300'
  if (line.startsWith('-')) return 'bg-red-50 text-red-800 dark:bg-red-950/60 dark:text-red-300'
  return 'text-zinc-500'
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && !busy.value) close()
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="fixed inset-0 z-[60] grid place-items-center bg-black/30 p-4 dark:bg-black/60" @click.self="!busy && close()">
    <div class="flex max-h-[85vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-2xl dark:border-zinc-700 dark:bg-zinc-900" role="dialog" aria-modal="true">
      <div class="border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
        <h2 class="font-semibold">{{ result ? 'Done' : plan?.title ?? requests[0]?.label ?? 'Change' }}</h2>
        <p v-if="!result" class="mt-0.5 text-xs text-zinc-500">Nothing changes until you apply. Each file is backed up first.</p>
      </div>

      <div class="flex-1 space-y-4 overflow-y-auto px-5 py-4 text-sm">
        <div v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-red-800 dark:bg-red-950 dark:text-red-200">{{ error }}</div>
        <div v-if="!plan && !error" class="text-zinc-400">Working out the change…</div>

        <template v-if="result">
          <p>Changed {{ result.changed.length === 1 ? 'one file' : `${result.changed.length} files` }}. The page shows the new state.</p>
          <div v-if="result.backups?.length">
            <div class="mb-1 text-xs text-zinc-500">Backups</div>
            <ul class="space-y-0.5 font-mono text-xs">
              <li v-for="b in result.backups" :key="b" class="break-all">{{ tilde(b) }}</li>
            </ul>
          </div>
        </template>

        <template v-else-if="plan">
          <details v-if="plan.skipped?.length" class="rounded-md bg-zinc-50 px-3 py-2 text-xs text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
            <summary class="cursor-pointer">{{ plan.skipped.length }} of {{ requests.length }} no longer apply and are left out</summary>
            <ul class="mt-1.5 space-y-0.5">
              <li v-for="(s, i) in plan.skipped" :key="i">{{ s.reason }}</li>
            </ul>
          </details>
          <div v-for="c in plan.changes" :key="c.path">
            <div class="mb-1.5 flex items-center gap-2 text-xs">
              <span class="rounded bg-zinc-100 px-1.5 py-0.5 font-medium dark:bg-zinc-800">{{ c.op === 'remove' ? 'remove' : 'edit' }}</span>
              <span class="truncate font-mono">{{ tilde(c.path) }}</span>
            </div>
            <pre class="overflow-x-auto rounded-md border border-zinc-200 py-2 font-mono text-[12px] leading-relaxed dark:border-zinc-800"><div v-for="(line, i) in c.diff.trimEnd().split('\n')" :key="i" class="px-3 whitespace-pre" :class="lineClass(line)">{{ line || ' ' }}</div></pre>
          </div>
        </template>
      </div>

      <div class="flex justify-end gap-2 border-t border-zinc-200 px-5 py-3 dark:border-zinc-800">
        <button class="rounded-md px-3 py-1.5 text-sm font-medium text-zinc-600 hover:bg-zinc-100 disabled:opacity-50 dark:text-zinc-300 dark:hover:bg-zinc-800" :disabled="busy" @click="close">
          {{ result ? 'Close' : 'Cancel' }}
        </button>
        <button
          v-if="!result"
          class="rounded-md bg-accent-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-accent-700 disabled:opacity-50"
          :disabled="!plan || busy"
          @click="apply"
        >
          {{ busy ? 'Applying…' : 'Apply' }}
        </button>
      </div>
    </div>
  </div>
</template>
