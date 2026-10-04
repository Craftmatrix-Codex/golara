type Invocation = {
  method: string
  url: string
  headers: Record<string, string[]>
  body: string
  function: string
  project: string
}

type FunctionHandler = (request: Request) => Response | Promise<Response>

const encoder = new TextEncoder()
const decoder = new TextDecoder()
const writeStderr = async (...values: unknown[]) => {
  const message = values
    .map((value) => (typeof value === 'string' ? value : JSON.stringify(value)))
    .join(' ')
  await Deno.stderr.write(encoder.encode(`${message}\n`))
}
console.log = (...values) => void writeStderr(...values)
console.info = (...values) => void writeStderr(...values)
console.warn = (...values) => void writeStderr(...values)
console.error = (...values) => void writeStderr(...values)

let servedHandler: FunctionHandler | undefined
const captureServe = (
  optionsOrHandler: FunctionHandler | Record<string, unknown>,
  maybeHandler?: FunctionHandler
) => {
  servedHandler = typeof optionsOrHandler === 'function' ? optionsOrHandler : maybeHandler
  return {
    addr: { hostname: '127.0.0.1', port: 0, transport: 'tcp' },
    finished: Promise.resolve(),
    ref() {},
    shutdown: async () => {},
    unref() {},
  }
}
Object.defineProperty(Deno, 'serve', { configurable: true, value: captureServe })

const entrypoint = Deno.args[0]
if (!entrypoint) throw new Error('function entrypoint is required')
const module = await import(new URL(`file://${entrypoint}`).href)
const handler = servedHandler ?? module.default
if (typeof handler !== 'function') {
  throw new Error('function must export a default handler or call Deno.serve(handler)')
}

const input = JSON.parse(await new Response(Deno.stdin.readable).text()) as Invocation
const headers = new Headers()
for (const [name, values] of Object.entries(input.headers ?? {})) {
  for (const value of values) headers.append(name, value)
}
const bytes = Uint8Array.from(atob(input.body ?? ''), (character) => character.charCodeAt(0))
const method = input.method || 'GET'
const request = new Request(new URL(input.url, 'http://localhost'), {
  method,
  headers,
  body: method === 'GET' || method === 'HEAD' ? undefined : bytes,
})
const response = await handler(request)
if (!(response instanceof Response)) throw new Error('function handler must return a Response')
const responseBytes = new Uint8Array(await response.arrayBuffer())
let binary = ''
for (let offset = 0; offset < responseBytes.length; offset += 0x8000) {
  binary += String.fromCharCode(...responseBytes.subarray(offset, offset + 0x8000))
}
const responseHeaders: Record<string, string[]> = {}
response.headers.forEach((value, name) => {
  responseHeaders[name] = [...(responseHeaders[name] ?? []), value]
})
const output = JSON.stringify({
  status: response.status,
  headers: responseHeaders,
  body: btoa(binary),
})
await Deno.stdout.write(encoder.encode(`${output}\n`))
