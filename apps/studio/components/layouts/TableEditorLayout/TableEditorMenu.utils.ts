export function getEntityNames(
  entities: ({ name?: unknown } | null | undefined)[] | undefined
): string[] {
  return (entities ?? [])
    .map((entity) => entity?.name)
    .filter((name): name is string => typeof name === 'string' && name.length > 0)
}
