<script setup lang="ts">
import { computed } from 'vue'
import { tilde, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import ScopeBadge from '../components/ScopeBadge.vue'
import StatusDot from '../components/StatusDot.vue'
import Empty from '../components/Empty.vue'

const { config } = useConfig()
const rows = computed(() => config.value?.mcpServers ?? [])
</script>

<template>
  <PageHeader
    title="MCP servers"
    subtitle="Configured servers and what they connect to. Arguments, environment variables and headers are never read, because they often hold credentials. With the same name, local beats project beats user."
  />
  <Empty v-if="!rows.length">No MCP servers configured.</Empty>
  <div v-else class="overflow-x-auto rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
    <table class="w-full text-sm">
      <thead class="border-b border-zinc-200 text-left text-xs text-zinc-500 dark:border-zinc-800">
        <tr>
          <th class="px-4 py-2 font-medium">Name</th>
          <th class="px-4 py-2 font-medium">Transport</th>
          <th class="px-4 py-2 font-medium">Target</th>
          <th class="px-4 py-2 font-medium">Source</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-zinc-100 dark:divide-zinc-800">
        <tr v-for="s in rows" :key="s.name + s.path" :class="s.status !== 'active' && 'opacity-60'">
          <td class="px-4 py-2.5">
            <div class="flex items-center gap-2"><StatusDot :status="s.status" :note="s.statusNote" /><span class="font-mono text-[13px]">{{ s.name }}</span></div>
            <div v-if="s.statusNote" class="ml-4 text-xs text-zinc-500">{{ s.statusNote }}</div>
          </td>
          <td class="px-4 py-2.5 text-zinc-500">{{ s.transport }}</td>
          <td class="max-w-md px-4 py-2.5"><code class="font-mono text-xs break-all">{{ tilde(s.target) }}</code></td>
          <td class="px-4 py-2.5">
            <ScopeBadge :source="s.source" />
            <div class="mt-0.5 font-mono text-[11px] text-zinc-400">{{ tilde(s.path).split('/').slice(-2).join('/') }}</div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
