const application = 'poslsmflcfylqkhdl131pj6o'
const volume = `${application}-golara-stable-data`

export function assertStableAttestation(containers, revision, initial) {
  if (!/^[a-f0-9]{40}$/.test(revision)) throw new Error('Expected revision must be a full Git SHA')
  if (!Array.isArray(containers) || containers.length !== 1)
    throw new Error('Stable must have exactly one running application container')
  const container = containers[0]
  if (
    !/^[a-f0-9]{64}$/.test(container.id ?? '') ||
    !container.name?.startsWith(`${application}-`) ||
    container.image !== `${application}:${revision}` ||
    container.health !== 'healthy' ||
    container.volume !== volume
  )
    throw new Error(
      'Stable image, health, or persistent volume does not match the expected deployment'
    )
  if (initial && (container.name !== initial.name || container.id !== initial.id))
    throw new Error('Stable container changed during verification')
  return container
}

export function redactLog(log, config) {
  const sensitive = Object.entries(config)
    .filter(
      ([name, value]) =>
        /key|password|token|username/i.test(name) && typeof value === 'string' && value
    )
    .map(([, value]) => value)
    .sort((a, b) => b.length - a.length)
  return sensitive.reduce((text, value) => text.split(value).join('[REDACTED]'), log)
}

export function summarizeStage(stage, raw) {
  const log = raw.replace(/\x1b\[[0-9;]*m/g, '')
  if (stage === 'sdk') {
    const value = (name) => {
      const match = log.match(new RegExp(`^# ${name} (\\d+)$`, 'm'))
      if (!match) throw new Error(`Missing SDK ${name} count`)
      return Number(match[1])
    }
    const total = value('tests'),
      passed = value('pass')
    if (
      total < 16 ||
      passed !== total ||
      value('fail') ||
      value('skipped') ||
      value('cancelled') ||
      value('todo')
    )
      throw new Error('SDK suite did not run all required tests successfully')
    return { total, passed }
  }
  if (stage === 'browser') {
    const match = log.match(/\b(\d+) passed\b/)
    if (!match || Number(match[1]) < 18 || /\b\d+ (?:failed|skipped|flaky)\b/.test(log))
      throw new Error('Browser suite did not run all required tests successfully')
    return { total: Number(match[1]), passed: Number(match[1]) }
  }
  throw new Error('Unknown verification stage')
}
