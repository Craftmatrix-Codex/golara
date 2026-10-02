<?php

namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Response;
use Illuminate\Support\Facades\Http;
use PDO;
use PDOException;
use App\Services\PlatformTelemetry;
use App\Services\StudioContentStore;
use InvalidArgumentException;
use RuntimeException;

class StudioController
{
    public function health(): JsonResponse
    {
        return response()->json(['status' => 'ok']);
    }

    public function recordTelemetry(Request $request, PlatformTelemetry $telemetry): JsonResponse
    {
        $validated = $request->validate([
            'severity' => ['sometimes', 'string', 'max:16', 'in:debug,info,warning,error,fatal'],
            'source' => ['sometimes', 'string', 'max:32'],
            'event_type' => ['sometimes', 'string', 'max:64'],
            'project_id' => ['sometimes', 'nullable', 'string', 'max:128'],
            'request_id' => ['sometimes', 'nullable', 'string', 'max:128'],
            'route' => ['sometimes', 'nullable', 'string', 'max:512'],
            'method' => ['sometimes', 'nullable', 'string', 'max:16'],
            'status_code' => ['sometimes', 'nullable', 'integer', 'between:100,599'],
            'error_class' => ['sometimes', 'nullable', 'string', 'max:255'],
            'message' => ['required', 'string', 'max:8192'],
            'stack_trace' => ['sometimes', 'nullable', 'string', 'max:32768'],
            'context' => ['sometimes', 'array'],
            'release' => ['sometimes', 'nullable', 'string', 'max:128'],
        ]);

        $validated['source'] ??= 'browser';
        $validated['event_type'] ??= 'error';
        $validated['route'] ??= $request->path();
        $validated['method'] ??= $request->method();
        $validated['user_agent'] = $request->userAgent();
        $validated['request_id'] ??= $request->header('X-Request-ID');

        $id = $telemetry->record($validated);

        return response()->json(['accepted' => $id !== null, 'id' => $id], Response::HTTP_ACCEPTED);
    }

    public function telemetryEvents(Request $request, PlatformTelemetry $telemetry): JsonResponse
    {
        $validated = $request->validate([
            'severity' => ['sometimes', 'string', 'max:16'],
            'source' => ['sometimes', 'string', 'max:32'],
            'event_type' => ['sometimes', 'string', 'max:64'],
            'project_id' => ['sometimes', 'string', 'max:128'],
            'limit' => ['sometimes', 'integer', 'between:1,100'],
        ]);

        return response()->json([
            'data' => $telemetry->recent($validated, (int) ($validated['limit'] ?? 100)),
        ]);
    }

    public function utcTime(): JsonResponse
    {
        return response()->json(['utcTime' => now('UTC')->toIso8601String()]);
    }

    public function apiNotFound(): JsonResponse
    {
        return response()->json([
            'error' => ['message' => 'Studio API endpoint not implemented'],
        ], 404);
    }

    public function apiRest(string $project): JsonResponse
    {
        $record = $this->projectRecord($project);
        $endpoint = $this->projectEndpointParts($record);

        return response()->json([
            'swagger' => '2.0',
            'info' => [
                'title' => $record['name'] . ' REST API',
                'description' => 'Supadata-compatible REST API schema',
                'version' => '1.0.0',
            ],
            'host' => $endpoint['host'],
            'basePath' => '/rest/v1',
            'schemes' => [$endpoint['protocol']],
            'consumes' => ['application/json'],
            'produces' => ['application/json'],
            'paths' => [],
            'definitions' => [],
            'parameters' => [],
            'externalDocs' => [
                'description' => 'Supadata REST API documentation',
                'url' => rtrim($this->projectEndpoint($record), '/') . '/docs',
            ],
        ]);
    }

    public function aiSqlCheckApiKey(): JsonResponse
    {
        return response()->json(['hasKey' => false]);
    }

    public function jwtSecretUpdateStatus(string $project): JsonResponse
    {
        $this->assertProject($project);

        return response()->json(['update_status' => null]);
    }

    public function postgrestConfig(string $project): JsonResponse
    {
        $this->assertProject($project);

        return response()->json([
            'db_anon_role' => 'anon',
            'db_extra_search_path' => env('PGRST_DB_EXTRA_SEARCH_PATH', 'public'),
            'db_schema' => 'public',
            'jwt_secret' => '',
            'max_rows' => (int) env('PGRST_DB_MAX_ROWS', 1000),
            'role_claim_key' => '.role',
        ]);
    }

    public function analytics(string $project, string $name): JsonResponse
    {
        $this->assertProject($project);
        abort_unless($name !== '', 404);

        return response()->json(['result' => []]);
    }

    public function organizations(): JsonResponse
    {
        return response()->json([]);
    }

    public function project(string $project): JsonResponse
    {
        $record = $this->projectRecord($project);
        $restUrl = $this->projectRestUrl($record);
        unset($record['scope']);

        return response()->json(array_merge($record, [
            'connectionString' => '',
            'restUrl' => $restUrl,
        ]));
    }

    public function profile(): JsonResponse
    {
        $project = $this->projectRecord('default');
        $publicProject = $project;
        unset($publicProject['scope']);

        return response()->json([
            'id' => 1,
            'primary_email' => 'johndoe@supabase.io',
            'username' => 'johndoe',
            'first_name' => 'John',
            'last_name' => 'Doe',
            'organizations' => [[
                'id' => 1,
                'name' => env('DEFAULT_ORGANIZATION_NAME', 'Default Organization'),
                'slug' => 'default-org-slug',
                'billing_email' => 'billing@supabase.co',
                'projects' => [array_merge($publicProject, ['connectionString' => ''])],
            ]],
        ]);
    }

    public function deploymentCommit(): JsonResponse
    {
        return response()->json([
            'commitSha' => env('VERCEL_GIT_COMMIT_SHA', 'development'),
            'commitTime' => 'unknown',
        ]);
    }

    public function cliReleaseVersion(): JsonResponse
    {
        return response()->json([
            'current' => env('CURRENT_CLI_VERSION') ? 'v' . env('CURRENT_CLI_VERSION') : null,
        ]);
    }

    public function featureOverrides(): JsonResponse
    {
        return response()->json(['disabled_features' => []]);
    }

    public function deploymentMode(): JsonResponse
    {
        return response()->json(['is_cli_mode' => false]);
    }

    public function databases(string $project): JsonResponse
    {
        $record = $this->projectRecord($project);
        $database = $record['scope']['database'] ?? [];
        $connection = $this->databaseConnectionDetails();

        return response()->json([[
            'cloud_provider' => $connection['host'] !== '' ? 'configured' : 'unconfigured',
            'connectionString' => $connection['connectionString'],
            'connection_string_read_only' => $connection['connectionString'],
            'db_host' => $connection['host'],
            'db_name' => $database['name'] ?? $connection['database'],
            'db_port' => $connection['port'],
            'db_user' => $database['role'] ?? $connection['user'],
            'identifier' => $project,
            'inserted_at' => '',
            'region' => 'configured',
            'restUrl' => $this->projectRestUrl($record),
            'size' => '',
            'status' => $connection['host'] !== '' ? 'ACTIVE_HEALTHY' : 'NOT_CONFIGURED',
        ]]);
    }

    public function apiKeys(string $project): JsonResponse
    {
        $this->assertProject($project);

        return response()->json([]);
    }

    public function temporaryApiKey(string $project): JsonResponse
    {
        $this->assertProject($project);

        return response()->json([
            'api_key' => (string) env('SUPABASE_SERVICE_KEY', env('SERVICE_ROLE_KEY', '')),
        ]);
    }

    public function githubAuthorization(): Response
    {
        return new Response('null', 200, ['Content-Type' => 'application/json']);
    }

    public function projects(): JsonResponse
    {
        return response()->json(array_map(fn (array $project): array => $this->publicProject($project), $this->projectRecords()));
    }

    public function currentProject(): JsonResponse
    {
        $project = collect($this->projectRecords())->firstWhere('current', true);

        return response()->json(['project' => is_array($project) ? $this->publicProject($project) : null]);
    }

    public function runLints(string $project): JsonResponse
    {
        $this->projectRecord($project);

        return response()->json([]);
    }

    public function graphql(Request $request, string $project): Response
    {
        $this->assertProject($project);
        $endpoint = rtrim((string) env('SUPABASE_URL', env('SUPADATA_PUBLIC_URL', '')), '/') . '/graphql/v1';
        abort_unless($endpoint !== '/graphql/v1', 503, 'GraphQL endpoint is not configured');

        $headers = [
            'Accept' => 'application/json',
            'Content-Type' => 'application/json',
            'apikey' => (string) env('SUPABASE_ANON_KEY', env('ANON_KEY', '')),
        ];
        foreach (['Authorization', 'x-graphql-authorization'] as $header) {
        if ($header === 'Authorization' && ! str_starts_with((string) $request->header($header), 'Bearer ')) continue;
            if ($request->hasHeader($header)) $headers[$header] = $request->header($header);
        }

        $upstream = Http::timeout(30)->withHeaders($headers)->post($endpoint, $request->json()->all());
        return response($upstream->body(), $upstream->status())
            ->header('Content-Type', $upstream->header('Content-Type', 'application/json'));
    }

    public function pgMetaQuery(Request $request, string $project): JsonResponse
    {
        $this->assertProject($project);
        $query = $request->input('query');
        abort_unless(is_string($query) && trim($query) !== '', 422, 'query is required');

        $key = $request->query('key');
        $connectionString = (string) env('SUPADATA_DATABASE_URL', env('DATABASE_URL', ''));
        if ($connectionString === '') {
            if (is_string($key) && str_starts_with($key, 'entity-types-')) {
                return response()->json([['data' => ['entities' => [], 'count' => 0]]]);
            }

            return response()->json([]);
        }

        try {
            $parsed = parse_url($connectionString);
            if (! is_array($parsed) || ! isset($parsed['host'], $parsed['path'])) {
                throw new RuntimeException('invalid database connection URL');
            }

            $dsn = sprintf(
                'pgsql:host=%s;port=%d;dbname=%s',
                $parsed['host'],
                (int) ($parsed['port'] ?? 5432),
                ltrim((string) $parsed['path'], '/'),
            );
            $pdo = new PDO($dsn, rawurldecode((string) ($parsed['user'] ?? '')), rawurldecode((string) ($parsed['pass'] ?? '')), [
                PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
                PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
            ]);
            $statement = $pdo->query($query);
            if ($statement === false) return response()->json([]);

            $rows = $statement->fetchAll();
            return response()->json($rows);
        } catch (PDOException|RuntimeException $exception) {
            report($exception);
            return response()->json(['error' => ['message' => 'Database metadata query failed']], 502);
        }
    }

    public function content(Request $request, string $project, StudioContentStore $store): JsonResponse
    {
        $this->assertProject($project);
        try {
            if ($request->isMethod('put')) {
                return response()->json($store->upsertSnippet($project, $request->json()->all()));
            }
            if ($request->isMethod('delete')) {
                $rawIds = $request->query('ids', '');
                $ids = is_string($rawIds) ? array_values(array_filter(array_map('trim', explode(',', $rawIds)))) : [];
                abort_unless($ids !== [], 400, 'Snippet IDs are required');
                return response()->json($store->deleteSnippets($project, $ids));
            }
            return response()->json($store->listSnippets($project, $request->query()));
        } catch (InvalidArgumentException $exception) {
            return response()->json(['error' => $exception->getMessage()], 422);
        }
    }

    public function contentItem(string $project, string $id, StudioContentStore $store): JsonResponse
    {
        $this->assertProject($project);
        try {
            return response()->json($store->getSnippet($project, $id));
        } catch (RuntimeException) {
            return response()->json(['message' => 'Content not found.'], 404);
        }
    }

    public function contentFolders(Request $request, string $project, StudioContentStore $store): JsonResponse
    {
        $this->assertProject($project);
        try {
            if ($request->isMethod('post')) {
                return response()->json($store->createFolder($project, (string) $request->input('name')), 201);
            }
            return response()->json($store->listFolderContents($project, $request->query()));
        } catch (InvalidArgumentException $exception) {
            return response()->json(['error' => $exception->getMessage()], 422);
        }
    }

    public function contentFolder(string $project, string $id, StudioContentStore $store): JsonResponse
    {
        $this->assertProject($project);
        return response()->json($store->listFolderContents($project, ['folder_id' => $id]));
    }

    public function contentCount(string $project, StudioContentStore $store): JsonResponse
    {
        $this->assertProject($project);
        $content = $store->listSnippets($project, ['limit' => 1000]);
        return response()->json(['shared' => 0, 'favorites' => 0, 'private' => count($content['data'])]);
    }

    public function vectorBuckets(string $project): JsonResponse
    {
        $record = $this->projectRecord($project);
        $vectorBuckets = [];
        foreach ($this->projectBucketRecords($record) as $bucket) {
            if (strtoupper((string) ($bucket['type'] ?? '')) !== 'VECTOR') {
                continue;
            }
            $vectorBuckets[] = [
                'vectorBucketName' => $bucket['name'],
                'creationTime' => $bucket['created_at'],
            ];
        }

        return response()->json(['vectorBuckets' => $vectorBuckets]);
    }

    public function functions(string $project): JsonResponse
    {
        $record = $this->projectRecord($project);
        $functions = $record['functions'] ?? [];
        abort_unless(is_array($functions), 500, 'invalid function metadata');

        return response()->json(array_values(array_filter($functions, 'is_array')));
    }

    public function logDrains(string $project): JsonResponse
    {
        $record = $this->projectRecord($project);
        $logDrains = $record['log_drains'] ?? [];
        abort_unless(is_array($logDrains), 500, 'invalid log drain metadata');

        return response()->json(array_values(array_filter($logDrains, 'is_array')));
    }

    public function emptyArray(string $project): JsonResponse
    {
        $this->assertProject($project);

        return response()->json([]);
    }

    public function storageBuckets(Request $request, string $project): JsonResponse
    {
        $record = $this->projectRecord($project);
        $buckets = $this->projectBucketRecords($record);
        $search = trim((string) $request->query('search', ''));

        if ($search !== '') {
            $buckets = array_values(array_filter($buckets, function (array $bucket) use ($search): bool {
                return str_contains(strtolower($bucket['id']), strtolower($search))
                    || str_contains(strtolower($bucket['name']), strtolower($search));
            }));
        }

        $sortColumn = $request->query('sortColumn', 'created_at');
        $sortColumn = is_string($sortColumn) && in_array($sortColumn, ['id', 'name', 'updated_at', 'created_at'], true)
            ? $sortColumn
            : 'created_at';
        $sortOrder = $request->query('sortOrder', 'desc');
        $sortOrder = is_string($sortOrder) && in_array($sortOrder, ['asc', 'desc'], true)
            ? $sortOrder
            : 'desc';

        usort($buckets, function (array $left, array $right) use ($sortColumn, $sortOrder): int {
            $comparison = strcmp((string) $left[$sortColumn], (string) $right[$sortColumn]);
            if ($comparison === 0) {
                $comparison = strcmp($left['id'], $right['id']);
            }

            return $sortOrder === 'asc' ? $comparison : -$comparison;
        });

        $limit = $this->boundedQueryInteger($request->query('limit'), 100, 1, 1000);
        $offset = $this->boundedQueryInteger($request->query('offset'), 0, 0, PHP_INT_MAX);

        return response()->json(array_values(array_slice($buckets, $offset, $limit)));
    }

    /** @return array<int, array<string, mixed>> */
    private function projectBucketRecords(array $record): array
    {
        $storage = $record['scope']['storage'] ?? [];
        $configured = is_array($storage) && is_array($storage['buckets'] ?? null)
            ? $storage['buckets']
            : (is_array($record['buckets'] ?? null) ? $record['buckets'] : []);

        if ($configured === []
            && in_array($record['status'] ?? null, ['ready', 'provisioned', 'ACTIVE_HEALTHY'], true)
            && is_array($storage)
            && is_string($storage['bucket'] ?? null)
            && $storage['bucket'] !== '') {
            $timestamp = is_string($record['inserted_at'] ?? null) && $record['inserted_at'] !== ''
                ? $record['inserted_at']
                : now()->toIso8601String();
            $configured[] = [
                'id' => $storage['bucket'],
                'name' => $storage['bucket'],
                'owner' => '',
                'public' => false,
                'type' => 'STANDARD',
                'created_at' => $timestamp,
                'updated_at' => $timestamp,
            ];
        }

        $result = [];
        foreach ($configured as $bucket) {
            if (! is_array($bucket) || ! is_string($bucket['id'] ?? null) || $bucket['id'] === '') {
                continue;
            }

            $createdAt = is_string($bucket['created_at'] ?? null) && $bucket['created_at'] !== ''
                ? $bucket['created_at']
                : (string) ($record['inserted_at'] ?? now()->toIso8601String());
            $updatedAt = is_string($bucket['updated_at'] ?? null) && $bucket['updated_at'] !== ''
                ? $bucket['updated_at']
                : $createdAt;
            $normalized = [
                'id' => $bucket['id'],
                'name' => is_string($bucket['name'] ?? null) && $bucket['name'] !== '' ? $bucket['name'] : $bucket['id'],
                'owner' => is_string($bucket['owner'] ?? null) ? $bucket['owner'] : '',
                'public' => (bool) ($bucket['public'] ?? false),
                'created_at' => $createdAt,
                'updated_at' => $updatedAt,
            ];
            if (is_array($bucket['allowed_mime_types'] ?? null)) {
                $normalized['allowed_mime_types'] = array_values(array_filter($bucket['allowed_mime_types'], 'is_string'));
            }
            if (is_numeric($bucket['file_size_limit'] ?? null) && (int) $bucket['file_size_limit'] >= 0) {
                $normalized['file_size_limit'] = (int) $bucket['file_size_limit'];
            }
            if (is_string($bucket['type'] ?? null) && $bucket['type'] !== '') {
                $normalized['type'] = $bucket['type'];
            }
            $result[] = $normalized;
        }

        return $result;
    }

    private function boundedQueryInteger(mixed $value, int $default, int $minimum, int $maximum): int
    {
        if (is_array($value)) {
            $value = $value[0] ?? null;
        }
        if (! is_string($value) && ! is_int($value)) {
            return $default;
        }
        $parsed = filter_var($value, FILTER_VALIDATE_INT);
        if ($parsed === false) {
            return $default;
        }

        return min($maximum, max($minimum, (int) $parsed));
    }

    public function settings(string $project): JsonResponse
    {
        $record = $this->projectRecord($project);
        $database = $record['scope']['database'] ?? [];
        $endpoint = $this->projectEndpointParts($record, $project);
        $connection = $this->databaseConnectionDetails();

        return response()->json([
            'app_config' => [
                'db_schema' => 'public',
                'endpoint' => $endpoint['host'],
                'storage_endpoint' => $endpoint['host'],
                'protocol' => $endpoint['protocol'],
            ],
            'cloud_provider' => $connection['host'] !== '' ? 'configured' : 'unconfigured',
            'db_dns_name' => $connection['host'],
            'db_host' => $connection['host'],
            'connectionString' => $connection['connectionString'],
            'db_ip_addr_config' => 'configured',
            'db_name' => $database['name'] ?? $connection['database'],
            'db_port' => $connection['port'],
            'db_user' => $database['role'] ?? $connection['user'],
            'inserted_at' => $record['inserted_at'] ?? now()->toIso8601String(),
            'jwt_secret' => '',
            'name' => $record['name'],
            'ref' => $project,
            'region' => 'configured',
            'pooler_tenant_id' => $this->poolerTenantId(),
            'service_api_keys' => [
                ['api_key' => '', 'name' => 'anon key', 'tags' => 'anon'],
                ['api_key' => '', 'name' => 'service_role key', 'tags' => 'service_role'],
            ],
            'ssl_enforced' => false,
            'status' => $connection['host'] !== '' ? 'ACTIVE_HEALTHY' : 'NOT_CONFIGURED',
        ]);
    }

    public function revealDatabasePassword(Request $request, string $project): JsonResponse
    {
        $this->projectRecord($project);
        $validator = $request->input('validator');
        $configuredValidator = (string) config('studio.auth_password');

        if (! is_string($validator)
            || $configuredValidator === ''
            || ! hash_equals($configuredValidator, $validator)) {
            return response()->json(['message' => 'Validation failed'], 403);
        }

        $password = $this->databasePassword();
        if ($password === '') {
            return response()->json([
                'message' => 'Database password is unavailable. Configure SUPADATA_DATABASE_PASSWORD in Coolify and redeploy.',
            ], 409);
        }

        return response()
            ->json(['password' => $password])
            ->header('Cache-Control', 'no-store, private')
            ->header('Pragma', 'no-cache');
    }

    public function client(Request $request): Response
    {
        $shell = config('studio.client_shell');
        abort_unless(is_string($shell) && is_file($shell), 503, 'Studio client bundle is unavailable');

        return response(file_get_contents($shell), 200, [
            'Content-Type' => 'text/html; charset=UTF-8',
        ]);
    }

    private function projectRecord(string $project): array
    {
        $record = collect($this->projectRecords())->firstWhere('ref', $project);
        abort_unless(is_array($record), 404);

        return $record;
    }

    private function assertProject(string $project): void
    {
        $this->projectRecord($project);
    }

    private function publicProject(array $project): array
    {
        unset($project['scope']);
        return $project;
    }

    private function projectRecords(): array
    {
        $configured = array_values(array_filter(config('studio.projects', []), 'is_array'));
        $path = config('studio.registry_path');
        if (! is_string($path) || ! is_file($path)) {
            return $configured;
        }

        $decoded = json_decode((string) file_get_contents($path), true);
        if (! is_array($decoded) || ! is_array($decoded['projects'] ?? null)) {
            return $configured;
        }

        $currentId = is_string($decoded['currentProjectId'] ?? null) ? $decoded['currentProjectId'] : null;
        $records = [];
        foreach ($decoded['projects'] as $project) {
            if (! is_array($project) || ! is_string($project['id'] ?? null)) {
                continue;
            }
            $id = $project['id'];
            $records[] = [
                'id' => $id,
                'ref' => $id,
                'name' => is_string($project['name'] ?? null) ? $project['name'] : $id,
                'organization_id' => 1,
                'cloud_provider' => 'localhost',
                'status' => $this->nativeProjectStatus($project['status'] ?? null),
                'region' => 'local',
                'inserted_at' => $project['createdAt'] ?? '',
                'current' => $currentId !== null ? $id === $currentId : (bool) ($project['current'] ?? false),
                'scope' => is_array($project['scope'] ?? null) ? $project['scope'] : [],
            ];
        }

        $known = array_column($records, 'ref');
        foreach ($configured as $fallback) {
            if (! in_array($fallback['ref'] ?? null, $known, true)) {
                $fallback['current'] = $currentId !== null
                    ? ($fallback['ref'] ?? null) === $currentId
                    : (bool) ($fallback['current'] ?? false);
                $records[] = $fallback;
            }
        }

        return $records;
    }

    private function nativeProjectStatus(mixed $status): string
    {
        if (! is_string($status) || $status === '') {
            return 'UNKNOWN';
        }

        return match ($status) {
            'ready', 'provisioned' => 'ACTIVE_HEALTHY',
            'registered', 'provisioning', 'starting' => 'COMING_UP',
            'failed' => 'INIT_FAILED',
            default => $status,
        };
    }

    private function projectRestUrl(array $record): string
    {
        return rtrim($this->projectEndpoint($record), '/') . '/rest/v1';
    }

    private function databasePassword(): string
    {
        $configured = (string) (env('SUPADATA_DATABASE_PASSWORD') ?: env('DB_PASSWORD') ?: env('POSTGRES_PASSWORD'));
        if ($configured !== '') {
            return $configured;
        }

        $url = (string) (env('DATABASE_URL') ?: env('SUPADATA_DATABASE_URL') ?: config('database.connections.pgsql.url', ''));
        $parsed = $url !== '' ? parse_url($url) : false;

        return is_array($parsed) && is_string($parsed['pass'] ?? null)
            ? rawurldecode($parsed['pass'])
            : '';
    }

    private function poolerTenantId(): string
    {
        $tenantId = (string) (env('SUPADATA_POOLER_TENANT_ID') ?: env('POOLER_TENANT_ID'));

        return trim($tenantId);
    }

    private function databaseConnectionDetails(): array
    {
        $url = (string) (env('DATABASE_URL') ?: env('SUPADATA_DATABASE_URL') ?: config('database.connections.pgsql.url', ''));
        $parsed = $url !== '' ? parse_url($url) : false;
        $parsed = is_array($parsed) ? $parsed : [];

        $configuredHost = (string) (env('SUPADATA_PUBLIC_DATABASE_HOST') ?: env('DB_HOST') ?: env('POSTGRES_HOST'));
        $configuredPort = (int) (env('SUPADATA_PUBLIC_DATABASE_PORT') ?: env('DB_PORT') ?: env('POSTGRES_PORT') ?: 0);
        $configuredDatabase = (string) (env('SUPADATA_PUBLIC_DATABASE_NAME') ?: env('DB_DATABASE') ?: env('POSTGRES_DB'));
        $configuredUser = (string) (env('SUPADATA_PUBLIC_DATABASE_USER') ?: env('DB_USERNAME') ?: env('POSTGRES_USER'));
        $configHost = (string) config('database.connections.pgsql.host', '');
        $configPort = (int) config('database.connections.pgsql.port', 0);
        $configDatabase = (string) config('database.connections.pgsql.database', '');
        $configUser = (string) config('database.connections.pgsql.username', '');

        $host = (string) ($parsed['host'] ?? ($configuredHost !== '' ? $configuredHost : ($configHost !== 'localhost' ? $configHost : '')));
        $port = (int) ($parsed['port'] ?? ($configuredPort > 0 ? $configuredPort : ($configPort > 0 ? $configPort : 5432)));
        $database = rawurldecode((string) ($parsed['path'] ?? ''));
        $database = ltrim($database, '/');
        $database = $database !== '' ? $database : ($configuredDatabase !== '' ? $configuredDatabase : $configDatabase);
        $user = rawurldecode((string) ($parsed['user'] ?? ($configuredUser !== '' ? $configuredUser : $configUser)));
        $scheme = (string) ($parsed['scheme'] ?? 'postgresql');

        $connectionString = '';
        if ($host !== '' && $database !== '') {
            $displayHost = str_contains($host, ':') && ! str_starts_with($host, '[') ? '[' . $host . ']' : $host;
            $displayUser = $user !== '' ? rawurlencode($user) : 'postgres';
            $connectionString = sprintf(
                '%s://%s:%s@%s:%d/%s',
                $scheme,
                $displayUser,
                '[YOUR-PASSWORD]',
                $displayHost,
                $port > 0 ? $port : 5432,
                rawurlencode($database),
            );
        }

        return [
            'host' => $host,
            'port' => $port > 0 ? $port : 5432,
            'database' => $database,
            'user' => $user,
            'connectionString' => $connectionString,
        ];
    }

    private function projectEndpoint(array $record): string
    {
        $recordPublicUrl = $record['scope']['publicUrl'] ?? null;
        $configured = ($record['ref'] ?? null) === 'default'
            ? [
                env('SUPADATA_PUBLIC_URL'),
                env('SUPABASE_PUBLIC_URL'),
                config('app.url'),
                config('studio.project_endpoint'),
                $recordPublicUrl,
            ]
            : [
                $recordPublicUrl,
                env('SUPADATA_PUBLIC_URL'),
                env('SUPABASE_PUBLIC_URL'),
                config('app.url'),
                config('studio.project_endpoint'),
            ];

        foreach ($configured as $endpoint) {
            if (! is_string($endpoint) || trim($endpoint) === '') {
                continue;
            }

            $normalized = rtrim(trim($endpoint), '/');
            $host = parse_url($normalized, PHP_URL_HOST);
            if (is_string($host) && ! $this->isLocalEndpointHost($host)) {
                return $normalized;
            }
        }

        // Never present a container-local address as a live project endpoint.
        return '';
    }

    private function isLocalEndpointHost(string $host): bool
    {
        $normalized = strtolower(trim($host, '[]'));

        return $normalized === 'localhost'
            || $normalized === '127.0.0.1'
            || $normalized === '::1'
            || str_ends_with($normalized, '.localhost');
    }

    private function projectEndpointParts(array $record, ?string $project = null): array
    {
        $endpoint = $this->projectEndpoint($record);
        $parsed = parse_url($endpoint);

        if (is_array($parsed) && is_string($parsed['host'] ?? null)) {
            $host = $parsed['host'];
            if (isset($parsed['port'])) {
                $host .= ':' . $parsed['port'];
            }
            if (is_string($project) && trim($project) !== '') {
                $host .= '/api/projects/' . rawurlencode($project);
            }

            return [
                'host' => $host,
                'protocol' => is_string($parsed['scheme'] ?? null) ? $parsed['scheme'] : 'http',
            ];
        }

        return [
            'host' => preg_replace('#^https?://#', '', rtrim($endpoint, '/')),
            'protocol' => 'http',
        ];
    }
}
