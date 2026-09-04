import { Box, Check, ChevronsUpDown, Plus } from 'lucide-react'
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
  )
}
