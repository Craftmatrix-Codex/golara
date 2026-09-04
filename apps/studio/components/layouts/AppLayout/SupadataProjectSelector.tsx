import { Box, Check, ChevronsUpDown, Plus } from 'lucide-react'
import { useRouter } from 'next/router'
import { useEffect, useMemo, useState } from 'react'
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

const controlPlaneUrl = (
  process.env.NEXT_PUBLIC_SUPADATA_CONTROL_PLANE_URL || 'http://localhost:8090'
).replace(/\/$/, '')

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
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let isCancelled = false
    fetch(`${controlPlaneUrl}/api/projects`)
      .then(async (response) => {
        if (!response.ok) throw new Error('Failed to load Supadata projects')
        return (await response.json()) as { projects: SupadataProject[] }
      })
      .then(({ projects: nextProjects }) => {
        if (isCancelled) return
        setProjects(nextProjects)
        setSelectedId(nextProjects.find((project) => project.current)?.id ?? currentId ?? '')
        setError(null)
      })
      .catch((fetchError: unknown) => {
        if (!isCancelled)
          setError(fetchError instanceof Error ? fetchError.message : 'Failed to load projects')
      })
      .finally(() => {
        if (!isCancelled) setIsLoading(false)
      })

    return () => {
      isCancelled = true
    }
  }, [currentId])

  const selectedProject = projects.find((project) => project.id === selectedId)
  const displayName = selectedProject?.name ?? currentName

  async function selectProject(id: string) {
    if (!id || id === selectedId) {
      setOpen(false)
      return
    }

    setIsSelecting(true)
    setError(null)
    try {
      const response = await fetch(
        `${controlPlaneUrl}/api/projects/${encodeURIComponent(id)}/select`,
        { method: 'POST' }
      )
      if (!response.ok) throw new Error('Failed to switch Supadata project')
      setSelectedId(id)
      setOpen(false)
      await router.push(`/project/${id}`)
    } catch (selectError: unknown) {
      setError(selectError instanceof Error ? selectError.message : 'Failed to switch projects')
    } finally {
      setIsSelecting(false)
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
          disabled={isLoading || isSelecting || projects.length === 0}
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
              <CommandItem disabled className="gap-2 text-foreground-lighter">
                <Plus size={14} />
                <span>New project</span>
              </CommandItem>
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
