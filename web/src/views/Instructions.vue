<script setup lang="ts">
import { ref } from 'vue'
import type { Instruction } from '../types'
import { tilde, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import Drawer from '../components/Drawer.vue'
import InstructionRow from '../components/InstructionRow.vue'
import Empty from '../components/Empty.vue'

const { config, project } = useConfig()
const selected = ref<Instruction | null>(null)
</script>

<template>
  <PageHeader
    title="Instructions"
    :subtitle="project
      ? 'Instruction files in the order Claude Code reads them: managed, user, then every directory from the filesystem root down to the project, with local files last at each level. Nothing overrides; all of it is concatenated.'
      : 'Your user-level instruction files. Pick a project to see the full chain, including CLAUDE.md files in parent directories.'"
  />
  <Empty v-if="!config!.instructions.length">No instruction files found.</Empty>
  <ol v-else class="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
    <InstructionRow v-for="(i, n) in config!.instructions" :key="i.path" :item="i" :index="n + 1" :depth="0" @select="selected = $event" />
  </ol>

  <Drawer v-if="selected" :title="selected.import ?? tilde(selected.path).split('/').pop()!" :path="selected.exists ? selected.path : undefined" @close="selected = null">
    <p v-if="!selected.exists" class="text-red-600">This import does not resolve to a file: {{ selected.path }}</p>
    <p v-if="selected.conditional" class="text-zinc-500">This rule has a <code>paths</code> field, so it only loads when Claude works with a matching file.</p>
    <p v-if="selected.external" class="text-zinc-500">This import points outside the project. Claude Code loads it only after you approve external imports for the project.</p>
  </Drawer>
</template>
