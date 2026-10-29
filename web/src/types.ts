// Mirrors the JSON the Go API returns (internal/claude and internal/lint).

export type Scope = 'managed' | 'user' | 'project' | 'local' | 'plugin'
export type Status = 'active' | 'shadowed' | 'disabled'
export type Kind = 'agent' | 'skill' | 'command'
export type Severity = 'error' | 'warning' | 'info'

export interface Source {
  scope: Scope
  plugin?: string
}

export interface Usage {
  count: number
  lastUsed: string
}

export interface Component {
  kind: Kind
  name: string
  description: string
  source: Source
  path: string
  frontmatter?: Record<string, unknown>
  frontmatterError?: string
  error?: string
  usage?: Usage
  status: Status
  statusNote?: string
}

export interface Plugin {
  id: string
  name: string
  marketplace: string
  version?: string
  description?: string
  author?: string
  installPath?: string
  installScope?: string
  installedAt?: string
  updatedAt?: string
  installed: boolean
  enabled: boolean
  enabledBy?: Scope
  enabledIn?: string
  usage?: Usage
  counts: Record<string, number>
  error?: string
}

export interface Hook {
  event: string
  matcher?: string
  type: string
  detail: string
  timeout?: number
  source: Source
  path: string
  status: Status
}

export interface MCPServer {
  name: string
  transport: string
  target: string
  source: Source
  path: string
  status: Status
  statusNote?: string
}

export interface Instruction {
  scope: Scope
  kind: string
  path: string
  lines: number
  bytes: number
  error?: string
  exists: boolean
  conditional?: boolean
  import?: string
  line?: number
  external?: boolean
  appliesUnder?: string
  projects?: number
  imports?: Instruction[]
}

export interface MemoryEntry {
  name: string
  description: string
  type: string
  file: string
  path: string
  links: string[]
  bytes: number
  frontmatterError?: string
}

export interface MemoryStore {
  label: string
  dir: string
  kind: string
  project?: string
  index?: {
    path: string
    lines: number
    bytes: number
    links: { title: string; file: string; line: number; exists: boolean }[]
  }
  entries: MemoryEntry[]
}

export interface SettingsLayer {
  scope: Scope
  path: string
  exists: boolean
  error?: string
  ignored?: string
}

export interface SettingEntry {
  key: string
  value: unknown
  scope: Scope
  overridden?: Scope[]
  merged?: boolean
}

export interface Project {
  path: string
  exists: boolean
  hasMemory: boolean
}

export interface Config {
  project?: string
  projectRoot?: string
  settingsLayers: SettingsLayer[]
  settings: SettingEntry[]
  instructions: Instruction[]
  components: Component[]
  plugins: Plugin[] | null
  hooks: Hook[] | null
  mcpServers: MCPServer[] | null
  memory: MemoryStore[] | null
  projects: Project[] | null
  warnings?: string[]
}

export interface FixRequest {
  kind: 'skill-override' | 'plugin-enabled' | 'memory-index-add' | 'memory-index-remove' | 'remove-symlink'
  target: string
  value?: string
  line?: number
  label?: string
}

export interface FixPlan {
  requests: FixRequest[]
  title: string
  changes: { path: string; op: 'write' | 'remove'; diff: string; hash: string }[]
  skipped?: { request: FixRequest; reason: string }[]
}

export interface FixResult {
  changed: string[]
  backups: string[] | null
}

export interface Finding {
  rule: string
  severity: Severity
  message: string
  path?: string
  line?: number
  related?: string
  fix?: FixRequest
}

export interface Rule {
  id: string
  severity: Severity
  description: string
}

export interface Meta {
  version: string
  configDir: string
  globalConfig: string
  managedDir: string
  memoryDirs: string[] | null
  home: string
  readOnly: boolean
  rules: Rule[]
}
