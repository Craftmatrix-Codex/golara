<?php

namespace Tests\Feature;

use Tests\TestCase;

class StudioCompatibilityTest extends TestCase
{
    private function auth()
    {
        return $this->withBasicAuth('studio', 'password');
    }

    public function test_project_credentials_return_metadata_without_secret_values(): void
    {
        config()->set('studio.projects', [[
            'id' => 'default',
            'ref' => 'default',
            'name' => 'Default Project',
            'status' => 'ready',
            'inserted_at' => '2026-09-06T00:00:00Z',
        ]]);
        config()->set('studio.registry_path', '');
        config()->set('database.connections.pgsql.url', 'postgresql://runtime:secret@db.example.test:5432/app');

        $response = $this->auth()->getJson('/api/platform/projects/default/credentials');

        $response->assertOk()
            ->assertJsonStructure(['apiKey', 'deployablePassword', 'postgres'])
            ->assertJsonMissing(['secret'])
            ->assertJsonPath('postgres.host', 'db.example.test')
            ->assertJsonPath('postgres.port', 5432)
            ->assertJsonPath('postgres.database', 'app')
            ->assertJsonPath('postgres.username', 'runtime');
    }

    public function test_pg_meta_query_returns_a_successful_json_result(): void
    {
        $response = $this->auth()->postJson('/api/platform/pg-meta/default/query', [
            'query' => 'select 1',
        ]);

        $response->assertOk()->assertJsonIsArray();
    }

    public function test_pg_meta_entity_type_query_returns_native_data_row(): void
    {
        $this->auth()->postJson('/api/platform/pg-meta/default/query?key=entity-types-public-0', ['query' => 'select 1'])
            ->assertOk()
            ->assertJsonPath('0.data.entities', [])
            ->assertJsonPath('0.data.count', 0);
    }

    public function test_project_detail_uses_native_ref_identifier(): void
    {
        $this->auth()->getJson('/api/platform/projects/default')
            ->assertOk()
            ->assertJsonPath('ref', 'default');
    }

    public function test_project_rest_contract_returns_openapi_schema(): void
    {
        $this->auth()->getJson('/api/platform/projects/default/api/rest')
            ->assertOk()
            ->assertJsonPath('swagger', '2.0')
            ->assertJsonStructure([
                'basePath', 'consumes', 'definitions', 'externalDocs', 'host',
                'info', 'parameters', 'paths', 'produces', 'schemes',
            ]);
    }

    public function test_lints_accept_the_native_get_request(): void
    {
        $this->auth()->getJson('/api/platform/projects/default/run-lints')
            ->assertOk()
            ->assertExactJson([]);
    }

    public function test_content_and_content_count_return_empty_state_contracts(): void
    {
        $this->auth()->getJson('/api/platform/projects/default/content')
            ->assertOk()
            ->assertExactJson(['data' => [], 'cursor' => null]);

        $this->auth()->getJson('/api/platform/projects/default/content/count')
            ->assertOk()
            ->assertExactJson(['shared' => 0, 'favorites' => 0, 'private' => 0]);
    }

    public function test_content_folders_returns_the_native_empty_state_contract(): void
    {
        $this->auth()->getJson('/api/platform/projects/default/content/folders?type=sql&limit=100')
            ->assertOk()
            ->assertExactJson([
                'data' => ['folders' => [], 'contents' => []],
                'cursor' => null,
            ]);
    }

    public function test_temporary_api_key_contract_returns_native_shape(): void
    {
        $this->auth()->postJson('/api/platform/projects/default/api-keys/temporary?authorization_exp=3600&claims=%7B%22role%22%3A%22service_role%22%7D')
            ->assertOk()
            ->assertJsonStructure(['api_key']);
    }

    public function test_management_surfaces_return_native_empty_states(): void
    {
        $this->auth()->getJson('/api/platform/storage/default/vector-buckets')
            ->assertOk()
            ->assertExactJson(['vectorBuckets' => []]);

        foreach ([
            '/api/v1/projects/default/functions',
            '/api/platform/projects/default/analytics/log-drains',
        ] as $uri) {
            $this->auth()->getJson($uri)->assertOk()->assertExactJson([]);
        }
    }

    public function test_settings_uses_the_configured_database_endpoint_without_exposing_passwords(): void
    {
        config()->set('database.connections.pgsql.url', 'postgresql://shared_runtime:super-secret@203.0.113.42:6543/shared_db');

        $this->auth()->getJson('/api/platform/projects/default/settings')
            ->assertOk()
            ->assertJsonPath('db_host', '203.0.113.42')
            ->assertJsonPath('db_port', 6543)
            ->assertJsonPath('db_user', 'shared_runtime')
            ->assertJsonPath('db_dns_name', '203.0.113.42')
            ->assertJsonPath('status', 'ACTIVE_HEALTHY')
            ->assertJsonPath('connectionString', 'postgresql://shared_runtime:[YOUR-PASSWORD]@203.0.113.42:6543/shared_db')
            ->assertJsonMissing(['connectionString' => 'postgresql://shared_runtime:super-secret@203.0.113.42:6543/shared_db']);
    }

    public function test_settings_returns_the_native_studio_shape_without_secrets_by_default(): void
    {
        $this->auth()->getJson('/api/platform/projects/default/settings')
            ->assertOk()
            ->assertJsonStructure([
                'app_config' => ['db_schema', 'endpoint', 'storage_endpoint', 'protocol'],
                'name', 'ref', 'region', 'status', 'service_api_keys',
            ]);
    }

    public function test_database_password_reveal_requires_the_studio_validator(): void
    {
        config()->set('studio.auth_password', 'validator-secret');
        config()->set('database.connections.pgsql.url', 'postgresql://runtime:database-secret@db.example.test:5432/postgres');

        $this->withBasicAuth('studio', 'validator-secret')
            ->postJson('/api/platform/projects/default/connection-password', ['validator' => 'validator-secret'])
            ->assertOk()
            ->assertJsonPath('password', 'database-secret')
            ->assertHeader('Cache-Control', 'no-store, private');

        $this->withBasicAuth('studio', 'validator-secret')
            ->postJson('/api/platform/projects/default/connection-password', ['validator' => 'wrong-secret'])
            ->assertForbidden()
            ->assertJsonMissing(['password' => 'database-secret']);

        $this->withBasicAuth('studio', 'wrong-validator')
            ->postJson('/api/platform/projects/default/connection-password', ['validator' => 'validator-secret'])
            ->assertUnauthorized();
    }

    public function test_utc_time_endpoint_returns_native_json(): void
    {
        $this->auth()->getJson('/api/get-utc-time')
            ->assertOk()
            ->assertJsonStructure(['utcTime']);
    }

    public function test_unknown_api_paths_are_json_not_the_spa_shell(): void
    {
        $this->auth()->getJson('/api/not-implemented')
            ->assertNotFound()
            ->assertJsonPath('error.message', 'Studio API endpoint not implemented');
    }
}
