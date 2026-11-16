<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Component, Kind, Plugin } from '../types'
import { relativeTime, tilde, useConfig } from '../store'
import PageHeader from '../components/PageHeader.vue'
import SearchInput from '../components/SearchInput.vue'
import StatusDot from '../components/StatusDot.vue'
import Drawer from '../components/Drawer.vue'
import Field from '../components/Field.vue'
import Empty from '../components/Empty.vue'
import FixButton from '../components/FixButton.vue'

const props = defineProps<{ name: string }>()
const { project, config } = useConfig()

const search = ref('')
const selected = ref<Component | null>(null)
watch(() => props.name, () => {
  search.value = ''
  selected.value = null
})

const market = computed(() => config.value?.marketplaces?.find((m) => m.name === props.name))

const kinds: { kind: Kind; label: string }[] = [
  { kind: 'agent', label: 'Agents' },
  { kind: 'skill', label: 'Skills' },
  { kind: 'command', label: 'Commands' },
]

// Plugin components are namespaced plugin:name; under the plugin's own
// heading the prefix says nothing.
function shortName(c: Component, p: Plugin) {
  return c.name.startsWith(p.name + ':') ? c.name.slice(p.name.length + 1) : c.name
}

// One group per plugin, enabled ones first, each with its components by kind
// and its hooks and MCP servers.
const groups = computed(() => {
  const c = config.value
  const q = search.value.toLowerCase()
  const match = (...s: (string | undefined)[]) => !q || s.some((v) => v?.toLowerCase().includes(q))
  return (c?.plugins ?? [])
    .filter((p) => p.marketplace === props.name)
    .sort((a, b) => Number(b.enabled) - Number(a.enabled) || a.name.localeCompare(b.name))
    .map((p) => {
      const components = c?.components.filter((x) => x.source.plugin === p.id && match(x.name, x.description)) ?? []
      return {
        plugin: p,
        kinds: kinds
          .map((k) => ({ ...k, items: components.filter((x) => x.kind === k.kind).sort((a, b) => a.name.localeCompare(b.name)) }))
          .filter((k) => k.items.length),
        hooks: c?.hooks?.filter((h) => h.source.plugin === p.id && match(h.event, h.matcher, h.detail)) ?? [],
        servers: c?.mcpServers?.filter((s) => s.source.plugin === p.id && match(s.name, s.target)) ?? [],
      }
    })
    .filter((g) => !q || match(g.plugin.name, g.plugin.description) || g.kinds.length || g.hooks.length || g.servers.length)
})

const backLink = computed(() => ({ path: '/marketplaces', query: project.value ? { project: project.value } : {} }))
</script>

<template>
  <RouterLink :to="backLink" class="mb-2 inline-block text-xs text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100">← Marketplaces</RouterLink>
  <Empty v-if="!market">No marketplace named {{ name }} here.</Empty>
  <template v-else>
    <PageHeader :title="market.name" :subtitle="market.description">
      <SearchInput v-model="search" />
    </PageHeader>

    <dl class="mb-6 space-y-1.5 rounded-lg border border-zinc-200 bg-white px-4 py-3 text-sm dark:border-zinc-800 dark:bg-zinc-900">
      <Field v-if="market.location" label="Source">{{ market.source }} · <span class="font-mono text-xs">{{ market.location }}</span></Field>
      <Field v-if="market.owner" label="Owner">{{ market.owner }}</Field>
      <Field v-if="market.installLocation" label="Checkout"><span class="font-mono text-xs">{{ tilde(market.installLocation) }}</span></Field>
      <Field v-if="market.lastUpdated" label="Updated">{{ relativeTime(market.lastUpdated) }}<template v-if="market.autoUpdate"> · updates automatically</template></Field>
      <Field v-if="market.known" label="Plugins">{{ groups.length }} installed or enabled here, of {{ market.listed }} listed</Field>
      <Field v-if="market.error" label="Problem"><span class="text-red-600">{{ market.error }}</span></Field>
    </dl>

    <Empty v-if="!groups.length">{{ search ? 'Nothing matches.' : 'No plugin from this marketplace is installed or enabled here.' }}</Empty>
    <div class="space-y-4">
      <section
        v-for="g in groups"
        :key="g.plugin.id"
        class="overflow-hidden rounded-lg border bg-white dark:bg-zinc-900"
        :class="g.plugin.error && g.plugin.enabled ? 'border-red-300 dark:border-red-900' : 'border-zinc-200 dark:border-zinc-800'"
      >
        <header class="flex items-start justify-between gap-3 border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
          <div class="min-w-0">
            <h2 class="truncate font-mono text-[13px] font-semibold" :class="!g.plugin.enabled && 'text-zinc-500'">{{ g.plugin.name }}</h2>
            <div class="truncate text-xs text-zinc-400">
              <template v-if="g.plugin.version">{{ g.plugin.version }}</template><template v-if="g.plugin.author"> · {{ g.plugin.author }}</template>
            </div>
            <p v-if="g.plugin.description" class="mt-1 text-sm text-zinc-600 dark:text-zinc-400">{{ g.plugin.description }}</p>
            <p v-if="g.plugin.error" class="mt-1 text-sm text-red-600">{{ g.plugin.error }}</p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <span
              class="rounded px-1.5 py-0.5 text-[11px] font-medium"
              :class="g.plugin.enabled ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' : 'bg-zinc-100 text-zinc-500 dark:bg-zinc-800'"
              :title="g.plugin.enabledBy ? `decided by ${g.plugin.enabledBy} settings` : 'not mentioned in enabledPlugins'"
            >{{ g.plugin.enabled ? 'enabled' : 'off' }}<template v-if="g.plugin.enabledBy"> · {{ g.plugin.enabledBy }}</template></span>
            <FixButton
              v-if="g.plugin.enabledBy !== 'managed' && (g.plugin.enabled || g.plugin.installed)"
              :fix="{ kind: 'plugin-enabled', target: g.plugin.id, value: g.plugin.enabled ? 'false' : 'true', label: g.plugin.enabled ? 'Disable' : 'Enable' }"
            />
          </div>
        </header>

        <div class="grid gap-x-6 gap-y-4 px-4 py-3 sm:grid-cols-2 lg:grid-cols-3">
          <div v-for="k in g.kinds" :key="k.kind" class="min-w-0">
            <h3 class="mb-1 text-[11px] font-medium tracking-wide text-zinc-400 uppercase">{{ k.label }} <span class="tabular">{{ k.items.length }}</span></h3>
            <ul>
              <li v-for="c in k.items" :key="c.path + c.name">
                <button
                  class="flex w-full items-center gap-2 rounded px-1 py-0.5 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
                  :class="c.status !== 'active' && 'opacity-60'"
                  :title="c.error ?? c.description"
                  @click="selected = c"
                >
                  <StatusDot :status="c.status" :note="c.statusNote" />
                  <span class="truncate font-mono text-[13px]">{{ shortName(c, g.plugin) }}</span>
                </button>
              </li>
            </ul>
          </div>
          <div v-if="g.hooks.length" class="min-w-0">
            <h3 class="mb-1 text-[11px] font-medium tracking-wide text-zinc-400 uppercase">Hooks <span class="tabular">{{ g.hooks.length }}</span></h3>
            <ul>
              <li v-for="(h, i) in g.hooks" :key="i" class="flex items-center gap-2 px-1 py-0.5" :title="h.detail">
                <StatusDot :status="h.status" />
                <span class="truncate font-mono text-[13px]">{{ h.event }}<template v-if="h.matcher"> · {{ h.matcher }}</template></span>
              </li>
            </ul>
          </div>
          <div v-if="g.servers.length" class="min-w-0">
            <h3 class="mb-1 text-[11px] font-medium tracking-wide text-zinc-400 uppercase">MCP servers <span class="tabular">{{ g.servers.length }}</span></h3>
            <ul>
              <li v-for="s in g.servers" :key="s.name" class="flex items-center gap-2 px-1 py-0.5" :title="s.target">
                <StatusDot :status="s.status" :note="s.statusNote" />
                <span class="truncate font-mono text-[13px]">{{ s.name }}</span>
                <span class="shrink-0 text-xs text-zinc-400">{{ s.transport }}</span>
              </li>
            </ul>
          </div>
          <p v-if="!g.kinds.length && !g.hooks.length && !g.servers.length" class="text-sm text-zinc-400">Adds nothing dotclaude can list.</p>
        </div>
      </section>
    </div>

    <Drawer v-if="selected" :title="selected.name" :path="selected.error ? undefined : selected.path" @close="selected = null">
      <dl class="space-y-2">
        <Field label="Status">
          <span class="inline-flex items-center gap-2"><StatusDot :status="selected.status" />{{ selected.status }}</span>
          <div v-if="selected.statusNote" class="mt-0.5 text-xs text-zinc-500">{{ selected.statusNote }}</div>
        </Field>
        <Field label="Kind">{{ selected.kind }}</Field>
        <Field v-if="selected.description" label="Description">{{ selected.description }}</Field>
        <Field v-if="selected.usage" label="Used">{{ selected.usage.count }} times, last {{ relativeTime(selected.usage.lastUsed) }}</Field>
      </dl>
      <div v-if="selected.error" class="rounded-md bg-red-50 px-3 py-2 text-red-800 dark:bg-red-950 dark:text-red-200">{{ selected.error }}</div>
    </Drawer>
  </template>
</template>
