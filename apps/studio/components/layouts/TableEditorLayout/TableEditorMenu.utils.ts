export function getEntityNames(
  entities: ({ name?: unknown } | null | undefined)[] | undefined
): string[] {
  return (entities ?? [])
    .map((entity) => entity?.name)
    .filter((name): name is string => typeof name === 'string' && name.length > 0)
}

export function getValidEntities<T>(entities: (T | null | undefined)[] | undefined): T[] {
  return (entities ?? []).filter((entity): entity is T => entity !== null && entity !== undefined)
}
