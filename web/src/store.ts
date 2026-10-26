import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import type { Config, Finding, FixPlan, FixRequest, FixResult, Meta } from './types'

// One shared store: the configuration and lint findings for the project in
// the URL (?project=…), or the user-level view when there is none.

const config = ref<Config | null>(null)
const findings = ref<Finding[]>([])
const meta = ref<Meta | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
let loadedFor: string | null = null

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) throw new Error((await res.text()).trim() || res.statusText)
  return res.json() as Promise<T>
}

function query(project: string) {
  return project ? `?project=${encodeURIComponent(project)}` : ''
}

export async function load(project: string, force = false) {
  if (!force && loadedFor === project && config.value) return
  loading.value = true
  error.value = null
  try {
    const [c, f, m] = await Promise.all([
      getJSON<Config>(`/api/config${query(project)}`),
      getJSON<Finding[]>(`/api/lint${query(project)}`),
      meta.value ? Promise.resolve(meta.value) : getJSON<Meta>('/api/meta'),
    ])
    config.value = c
    findings.value = f
    meta.value = m
    loadedFor = project
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

export async function fetchFile(path: string, project: string): Promise<string> {
  const p = project ? `&project=${encodeURIComponent(project)}` : ''
  const res = await fetch(`/api/file?path=${encodeURIComponent(path)}${p}`)
  if (!res.ok) throw new Error((await res.text()).trim() || res.statusText)
  return res.text()
}

// Changes go through one dialog: a component asks for a fix, the dialog
// previews it and applies it on confirmation.
export const pendingFix = ref<{ request: FixRequest; project: string } | null>(null)

export function requestFix(request: FixRequest, project: string) {
  pendingFix.value = { request, project }
}

export const canChange = computed(() => meta.value !== null && !meta.value.readOnly)

async function postJSON<T>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Dotclaude': '1' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error((await res.text()).trim() || res.statusText)
  return res.json() as Promise<T>
}

export function previewFix(request: FixRequest, project: string) {
  return postJSON<FixPlan>('/api/fix/preview', { request, project })
}

export async function applyFix(plan: FixPlan, project: string) {
  const hashes = Object.fromEntries(plan.changes.map((c) => [c.path, c.hash]))
  const result = await postJSON<FixResult>('/api/fix/apply', { request: plan.request, project, hashes })
  await load(project, true)
  return result
}

/** The configuration for the project in the current route. */
export function useConfig() {
  const route = useRoute()
  const project = computed(() => (typeof route.query.project === 'string' ? route.query.project : ''))
  watch(project, (p) => load(p), { immediate: true })
  return {
    project,
    config,
    findings,
    meta,
    loading,
    error,
    refresh: () => load(project.value, true),
  }
}

/** Shortens a path under the home directory to ~/… for display. */
export function tilde(path: string | undefined): string {
  if (!path) return ''
  const home = meta.value?.home
  return home && path.startsWith(home + '/') ? '~' + path.slice(home.length) : path
}

export function relativeTime(iso: string | undefined): string {
  if (!iso) return ''
  const t = new Date(iso).getTime()
  if (!t || t < 0) return ''
  const days = Math.round((Date.now() - t) / 86_400_000)
  if (days <= 0) return 'today'
  if (days === 1) return 'yesterday'
  if (days < 30) return `${days} days ago`
  if (days < 365) return `${Math.round(days / 30)} months ago`
  return `${Math.round(days / 365)} years ago`
}
