import { Box, Check, ChevronsUpDown, Eye, EyeOff, KeyRound, Plus, RefreshCw } from 'lucide-react'
import { useRouter } from 'next/router'
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  cn,
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Popover,
  PopoverContent,
  PopoverTrigger,
} from 'ui'

interface SupadataProject {
  id: string
  name: string
  status: string
  current?: boolean
}

interface SupadataCredentialMetadata {
  createdAt?: string | null
  host?: string
  port?: number
  database?: string
  username?: string
}

interface SupadataCredentials {
  apiKey?: SupadataCredentialMetadata
  deployablePassword?: SupadataCredentialMetadata
  postgres?: SupadataCredentialMetadata
}

interface RotatedCredential {
  type: string
  value: string
}

const proxyPath = '/api/supadata'

export function SupadataProjectSelector({
  currentId,
  currentName,
}: {
  currentId?: string
  currentName: string
}) {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const [projects, setProjects] = useState<SupadataProject[]>([])
  const [selectedId, setSelectedId] = useState(currentId ?? '')
  const [isLoading, setIsLoading] = useState(true)
  const [isSelecting, setIsSelecting] = useState(false)
  const [isCreating, setIsCreating] = useState(false)
  const [name, setName] = useState('')
  const [id, setId] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [credentialsOpen, setCredentialsOpen] = useState(false)
  const [credentials, setCredentials] = useState<SupadataCredentials | null>(null)
  const [isCredentialsLoading, setIsCredentialsLoading] = useState(false)
  const [rotatingType, setRotatingType] = useState<string | null>(null)
  const [rotatedCredential, setRotatedCredential] = useState<RotatedCredential | null>(null)
  const [isSecretVisible, setIsSecretVisible] = useState(false)

  const loadProjects = useCallback(async () => {
    setIsLoading(true)
    try {
      const response = await fetch(`${proxyPath}/projects`)
      if (!response.ok) throw new Error('Failed to load Supadata projects')
      const data = (await response.json()) as { projects: SupadataProject[] }
      setProjects(data.projects)
      setSelectedId(data.projects.find((project) => project.current)?.id ?? currentId ?? '')
      setError(null)
    } catch (fetchError: unknown) {
      setError(fetchError instanceof Error ? fetchError.message : 'Failed to load projects')
    } finally {
      setIsLoading(false)
    }
  }, [currentId])

  useEffect(() => {
    void loadProjects()
  }, [loadProjects])

  const loadCredentials = useCallback(async (projectId: string) => {
    setIsCredentialsLoading(true)
    try {
      const response = await fetch(
        `${proxyPath}/projects/${encodeURIComponent(projectId)}/credentials`
      )
      if (!response.ok) throw new Error('Failed to load project credentials')
      setCredentials((await response.json()) as SupadataCredentials)
    } catch (credentialsError: unknown) {
      setError(
        credentialsError instanceof Error
          ? credentialsError.message
          : 'Failed to load project credentials'
      )
    } finally {
      setIsCredentialsLoading(false)
    }
  }, [])

  async function rotateCredential(type: string) {
    if (!selectedId) return
    setRotatingType(type)
    setIsSecretVisible(false)
    setError(null)
    try {
      const response = await fetch(
        `${proxyPath}/projects/${encodeURIComponent(selectedId)}/credentials/${type}/rotate`,
        { method: 'POST' }
      )
      if (!response.ok) throw new Error('Failed to rotate credential')
      setRotatedCredential((await response.json()) as RotatedCredential)
      await loadCredentials(selectedId)
    } catch (rotateError: unknown) {
      setError(rotateError instanceof Error ? rotateError.message : 'Failed to rotate credential')
    } finally {
      setRotatingType(null)
    }
  }

  const selectedProject = projects.find((project) => project.id === selectedId)
  const displayName = selectedProject?.name ?? currentName

  async function selectProject(projectId: string) {
    if (!projectId || projectId === selectedId) {
      setOpen(false)
      return
    }
    setIsSelecting(true)
    setError(null)
    try {
      const response = await fetch(
        `${proxyPath}/projects/${encodeURIComponent(projectId)}/select`,
        {
          method: 'POST',
        }
      )
      if (!response.ok) throw new Error('Failed to switch Supadata project')
      setSelectedId(projectId)
      setOpen(false)
      await router.push(`/project/${projectId}`)
    } catch (selectError: unknown) {
      setError(selectError instanceof Error ? selectError.message : 'Failed to switch projects')
    } finally {
      setIsSelecting(false)
    }
  }

  async function createProject(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!name.trim()) return
    setIsCreating(true)
    setError(null)
    try {
      const response = await fetch(`${proxyPath}/projects`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), ...(id.trim() ? { id: id.trim() } : {}) }),
      })
      if (!response.ok) {
        const payload = (await response.json().catch(() => null)) as { error?: string } | null
        throw new Error(payload?.error ?? 'Failed to create Supadata project')
      }
      const payload = (await response.json()) as { project: SupadataProject }
      setName('')
      setId('')
      await loadProjects()
      setOpen(false)
      await router.push(`/project/${payload.project.id}`)
    } catch (createError: unknown) {
      setError(createError instanceof Error ? createError.message : 'Failed to create project')
    } finally {
      setIsCreating(false)
    }
  }

  const sortedProjects = useMemo(
    () => [...projects].sort((a, b) => a.name.localeCompare(b.name)),
    [projects]
  )

  return (
    <>
      <Popover open={open} onOpenChange={setOpen} modal={false}>
        <PopoverTrigger asChild>
          <Button
            variant="text"
            size="tiny"
            role="combobox"
            aria-expanded={open}
            aria-label="Select Supadata project"
            disabled={isLoading || isSelecting || isCreating}
            className="h-8 min-w-[170px] max-w-[260px] justify-between gap-3 px-2.5 text-sm font-normal"
            title={error ?? 'Select Supadata project'}
          >
            <span className="flex min-w-0 flex-1 items-center gap-2">
              <Box size={14} strokeWidth={1.5} className="shrink-0 text-foreground-lighter" />
              <span className="min-w-0 flex-1 truncate">{displayName}</span>
              <ChevronsUpDown size={14} className="shrink-0 text-foreground-lighter" />
            </span>
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[280px] p-0" align="start" sideOffset={6}>
          {isCreating ? (
            <form onSubmit={createProject} className="space-y-3 p-3">
              <div className="text-sm font-medium">New Supadata project</div>
              <input
                aria-label="Project name"
                autoFocus
                required
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="Project name"
                className="h-8 w-full rounded border bg-surface-100 px-2 text-sm"
              />
              <input
                aria-label="Project ID"
                value={id}
                onChange={(event) => setId(event.target.value)}
                placeholder="Project ID (optional)"
                className="h-8 w-full rounded border bg-surface-100 px-2 text-sm"
              />
              <div className="flex justify-end gap-2">
                <Button
                  type="button"
                  size="tiny"
                  variant="default"
                  onClick={() => setIsCreating(false)}
                >
                  Cancel
                </Button>
                <Button type="submit" size="tiny" loading={isCreating} disabled={!name.trim()}>
                  Create
                </Button>
              </div>
            </form>
          ) : (
            <Command>
              <CommandInput placeholder="Find project..." />
              <CommandList>
                <CommandEmpty>No projects found.</CommandEmpty>
                <CommandGroup heading="Supadata projects">
                  {sortedProjects.map((project) => (
                    <CommandItem
                      key={project.id}
                      value={`${project.name} ${project.id}`}
                      onSelect={() => selectProject(project.id)}
                      className="gap-2"
                    >
                      <Check
                        size={14}
                        className={cn(
                          'shrink-0',
                          project.id === selectedId ? 'opacity-100' : 'opacity-0'
                        )}
                      />
                      <span className="min-w-0 flex-1 truncate">{project.name}</span>
                      <span className="text-[11px] text-foreground-lighter">{project.status}</span>
                    </CommandItem>
                  ))}
                </CommandGroup>
                <CommandGroup>
                  {selectedId && (
                    <CommandItem
                      onSelect={() => {
                        setOpen(false)
                        setCredentialsOpen(true)
                        setRotatedCredential(null)
                        void loadCredentials(selectedId)
                      }}
                      className="gap-2"
                    >
                      <KeyRound size={14} />
                      <span>Credentials</span>
                    </CommandItem>
                  )}
                  <CommandItem onSelect={() => setIsCreating(true)} className="gap-2">
                    <Plus size={14} />
                    <span>New project</span>
                  </CommandItem>
                </CommandGroup>
              </CommandList>
            </Command>
          )}
          {error && <div className="px-3 pb-3 text-xs text-destructive">{error}</div>}
        </PopoverContent>
      </Popover>
      <Dialog open={credentialsOpen} onOpenChange={setCredentialsOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Project credentials</DialogTitle>
            <DialogDescription>
              View connection metadata and rotate project credentials.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 text-sm">
            {isCredentialsLoading && (
              <div className="text-foreground-lighter">Loading credentials…</div>
            )}
            {!isCredentialsLoading && credentials && (
              <>
                <CredentialRow
                  label="API key"
                  metadata={credentials.apiKey}
                  type="api-key"
                  isRotating={rotatingType === 'api-key'}
                  onRotate={rotateCredential}
                />
                <CredentialRow
                  label="Deployable password"
                  metadata={credentials.deployablePassword}
                  type="deployable-password"
                  isRotating={rotatingType === 'deployable-password'}
                  onRotate={rotateCredential}
                />
                <CredentialRow
                  label="Postgres password"
                  metadata={credentials.postgres}
                  type="postgres-password"
                  isRotating={rotatingType === 'postgres-password'}
                  onRotate={rotateCredential}
                />
              </>
            )}
            {rotatedCredential && (
              <div className="rounded-md border border-warning bg-warning-muted p-3">
                <div className="font-medium">Secret generated</div>
                <div className="mt-1 text-xs text-foreground-lighter">
                  It will not be shown again. Reveal it only when ready to copy it.
                </div>
                <div className="mt-2 flex items-center gap-2">
                  <code className="min-w-0 flex-1 truncate rounded bg-surface-200 px-2 py-1 text-xs">
                    {isSecretVisible ? rotatedCredential.value : '••••••••••••••••'}
                  </code>
                  <Button
                    type="button"
                    size="tiny"
                    variant="default"
                    aria-label={isSecretVisible ? 'Hide secret' : 'Reveal secret'}
                    onClick={() => setIsSecretVisible((visible) => !visible)}
                  >
                    {isSecretVisible ? <EyeOff size={14} /> : <Eye size={14} />}
                  </Button>
                </div>
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}

function CredentialRow({
  label,
  metadata,
  type,
  isRotating,
  onRotate,
}: {
  label: string
  metadata?: SupadataCredentialMetadata
  type: string
  isRotating: boolean
  onRotate: (type: string) => void
}) {
  const postgresAddress =
    metadata?.host && metadata.port ? `${metadata.host}:${metadata.port}` : null
  return (
    <div className="flex items-center justify-between gap-3 border-b pb-3 last:border-b-0 last:pb-0">
      <div className="min-w-0">
        <div className="font-medium">{label}</div>
        {postgresAddress && (
          <div className="truncate font-mono text-xs text-foreground-lighter">
            {postgresAddress}
          </div>
        )}
        {metadata?.database && (
          <div className="text-xs text-foreground-lighter">Database: {metadata.database}</div>
        )}
        {!postgresAddress && !metadata?.database && (
          <div className="text-xs text-foreground-lighter">Configured</div>
        )}
      </div>
      <Button
        type="button"
        size="tiny"
        variant="default"
        aria-label={`Rotate ${label}`}
        onClick={() => onRotate(type)}
        loading={isRotating}
      >
        <RefreshCw size={13} />
        <span>Rotate</span>
      </Button>
    </div>
  )
}
