import { loadConfig } from './config.mjs'

const cfg = loadConfig()
if (process.argv.includes('--sql-benchmark')) {
  console.log(`INSERT INTO public."${cfg.namespace}" (marker, name, quantity)
SELECT 'golara_sdk_benchmark_' || g, 'benchmark row ' || g, g
FROM generate_series(1, 2000) g ON CONFLICT (marker) DO NOTHING;`)
} else if (process.argv.includes('--sql')) {
  const table = `"${cfg.namespace}"`
  const rpc = `"${cfg.namespace}_echo"`
  console.log(`CREATE TABLE IF NOT EXISTS public.${table} (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), marker text NOT NULL UNIQUE, name text NOT NULL, quantity integer NOT NULL DEFAULT 0);
ALTER TABLE public.${table} ENABLE ROW LEVEL SECURITY;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.${table} TO service_role;
CREATE OR REPLACE FUNCTION public.${rpc}(message text) RETURNS TABLE(echo text) LANGUAGE sql STABLE AS 'SELECT message';
REVOKE ALL ON FUNCTION public.${rpc}(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.${rpc}(text) TO service_role;`)
} else {
  if (!cfg.studioUsername || !cfg.studioPassword)
    throw new Error('Studio credentials are required to deploy the disposable function fixture')
  const form = new FormData()
  form.set('metadata', JSON.stringify({ entrypoint_path: 'index.ts', verify_jwt: false }))
  form.append(
    'file[]',
    new Blob(
      [
        `Deno.serve(async (request) => {
  const body = await request.json()
  if (body.inspectEnvironment) return Response.json({url: Deno.env.get('SUPABASE_URL'), project: Deno.env.get('SUPABASE_PROJECT_REF'), requestURL: request.url})
  return Response.json(body)
})\n`,
      ],
      {
        type: 'application/typescript',
      }
    ),
    'index.ts'
  )
  const url = `${cfg.publicURL}/api/v1/projects/${cfg.project}/functions/deploy?slug=${cfg.namespace}-echo`
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      Authorization: `Basic ${Buffer.from(`${cfg.studioUsername}:${cfg.studioPassword}`).toString('base64')}`,
      Accept: 'application/json',
    },
    body: form,
  })
  if (response.status !== 201)
    throw new Error(`Disposable function fixture deployment returned HTTP ${response.status}`)
  console.log('Disposable SDK function fixture deployed.')
}
