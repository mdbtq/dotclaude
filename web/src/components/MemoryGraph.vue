<script setup lang="ts">
import { computed, ref } from 'vue'
import { forceCenter, forceCollide, forceLink, forceManyBody, forceSimulation, forceX, forceY } from 'd3-force'
import type { SimulationLinkDatum, SimulationNodeDatum } from 'd3-force'
import type { MemoryEntry } from '../types'

const props = defineProps<{ entries: MemoryEntry[] }>()
const emit = defineEmits<{ select: [path: string] }>()
const hovered = ref<string | null>(null)

interface Node extends SimulationNodeDatum {
  id: string
  label: string
  type: string
  path?: string
  degree: number
}

// The layout is computed once per set of entries and drawn statically: a
// settled force layout reads better than one that keeps moving.
const graph = computed(() => {
  const byKey = new Map<string, Node>()
  const nodes: Node[] = []
  for (const e of props.entries) {
    const n: Node = { id: e.path, label: e.name || e.file.replace(/\.md$/, ''), type: e.type, path: e.path, degree: 0 }
    nodes.push(n)
    byKey.set(n.label, n)
    byKey.set(e.file.replace(/\.md$/, ''), n)
  }
  const links: SimulationLinkDatum<Node>[] = []
  for (const e of props.entries) {
    for (const l of e.links) {
      let target = byKey.get(l)
      if (!target) {
        target = { id: `missing:${l}`, label: l, type: 'missing', degree: 0 }
        nodes.push(target)
        byKey.set(l, target)
      }
      links.push({ source: e.path, target: target.id })
    }
  }
  for (const l of links) {
    nodes.find((n) => n.id === l.source)!.degree++
    nodes.find((n) => n.id === l.target)!.degree++
  }

  forceSimulation(nodes)
    .force('link', forceLink<Node, SimulationLinkDatum<Node>>(links).id((d) => d.id).distance(60))
    .force('charge', forceManyBody().strength(-120))
    .force('center', forceCenter(0, 0))
    .force('x', forceX(0).strength(0.06))
    .force('y', forceY(0).strength(0.08))
    .force('collide', forceCollide(14))
    .stop()
    .tick(300)

  const xs = nodes.map((n) => n.x ?? 0)
  const ys = nodes.map((n) => n.y ?? 0)
  const pad = 60
  const minX = Math.min(0, ...xs) - pad
  const minY = Math.min(0, ...ys) - pad
  const w = Math.max(...xs, 0) + pad - minX
  const h = Math.max(...ys, 0) + pad - minY
  return { nodes, links: links as { source: Node; target: Node }[], viewBox: `${minX} ${minY} ${w} ${h}` }
})

const neighbours = computed(() => {
  const set = new Set<string>()
  if (!hovered.value) return set
  set.add(hovered.value)
  for (const l of graph.value.links) {
    if (l.source.id === hovered.value) set.add(l.target.id)
    if (l.target.id === hovered.value) set.add(l.source.id)
  }
  return set
})

const fill: Record<string, string> = {
  user: 'fill-sky-500',
  feedback: 'fill-violet-500',
  project: 'fill-emerald-500',
  reference: 'fill-amber-500',
  missing: 'fill-transparent stroke-zinc-400 [stroke-dasharray:2_2]',
}

function radius(n: Node) {
  return 4 + Math.min(n.degree, 8) * 0.9
}
</script>

<template>
  <div class="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
    <svg :viewBox="graph.viewBox" class="h-[70vh] w-full" role="img" aria-label="Links between memories">
      <g class="stroke-zinc-300 dark:stroke-zinc-700">
        <line
          v-for="(l, i) in graph.links"
          :key="i"
          :x1="l.source.x" :y1="l.source.y" :x2="l.target.x" :y2="l.target.y"
          stroke-width="1"
          :class="hovered && !(neighbours.has(l.source.id) && neighbours.has(l.target.id)) ? 'opacity-15' : ''"
        />
      </g>
      <g
        v-for="n in graph.nodes"
        :key="n.id"
        :transform="`translate(${n.x},${n.y})`"
        class="cursor-pointer"
        :class="hovered && !neighbours.has(n.id) ? 'opacity-25' : ''"
        @mouseenter="hovered = n.id"
        @mouseleave="hovered = null"
        @click="n.path && emit('select', n.path)"
      >
        <circle :r="radius(n)" :class="fill[n.type] ?? 'fill-zinc-400'" stroke-width="1" />
        <text
          :y="radius(n) + 11"
          text-anchor="middle"
          class="pointer-events-none fill-zinc-600 text-[9px] dark:fill-zinc-400"
          :class="n.degree < 2 && hovered !== n.id ? 'hidden' : ''"
        >{{ n.label }}</text>
      </g>
    </svg>
    <div class="flex flex-wrap gap-4 border-t border-zinc-200 px-4 py-2 text-xs text-zinc-500 dark:border-zinc-800">
      <span v-for="t in ['user', 'feedback', 'project', 'reference']" :key="t" class="flex items-center gap-1.5">
        <svg class="size-2.5" viewBox="0 0 10 10"><circle cx="5" cy="5" r="4" :class="fill[t]" /></svg>{{ t }}
      </span>
      <span class="flex items-center gap-1.5">
        <svg class="size-2.5" viewBox="0 0 10 10"><circle cx="5" cy="5" r="4" :class="fill.missing" /></svg>linked, not written yet
      </span>
      <span class="ml-auto">Labels show for memories with two or more links; hover for the rest.</span>
    </div>
  </div>
</template>
