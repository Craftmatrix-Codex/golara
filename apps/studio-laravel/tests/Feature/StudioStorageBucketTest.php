<?php

namespace Tests\Feature;

use Tests\TestCase;

class StudioStorageBucketTest extends TestCase
{
    public function test_bucket_listing_returns_native_metadata_for_the_requested_project(): void
    {
        $path = $this->writeRegistry([
            'currentProjectId' => 'alpha',
            'projects' => [[
                'id' => 'alpha',
                'name' => 'Alpha',
                'status' => 'ready',
                'current' => true,
                'createdAt' => '2026-09-06T00:00:00Z',
                'scope' => [
                    'storage' => [
                        'bucket' => 'supadata-alpha',
                        'buckets' => [[
                            'id' => 'avatars',
                            'name' => 'avatars',
                            'owner' => '',
                            'public' => true,
                            'allowed_mime_types' => ['image/png'],
                            'file_size_limit' => 10485760,
                            'type' => 'STANDARD',
                            'created_at' => '2026-09-06T00:00:01Z',
                            'updated_at' => '2026-09-06T00:00:02Z',
                        ]],
                    ],
                ],
            ]],
        ]);
        config(['studio.registry_path' => $path]);

        try {
            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/storage/alpha/buckets')
                ->assertOk()
                ->assertExactJson([[
                    'id' => 'avatars',
                    'name' => 'avatars',
                    'owner' => '',
                    'public' => true,
                    'allowed_mime_types' => ['image/png'],
                    'file_size_limit' => 10485760,
                    'type' => 'STANDARD',
                    'created_at' => '2026-09-06T00:00:01Z',
                    'updated_at' => '2026-09-06T00:00:02Z',
                ]]);
        } finally {
            @unlink($path);
        }
    }

    public function test_bucket_listing_applies_search_sort_and_pagination(): void
    {
        $path = $this->writeRegistry([
            'currentProjectId' => 'alpha',
            'projects' => [[
                'id' => 'alpha',
                'name' => 'Alpha',
                'status' => 'ready',
                'scope' => [
                    'storage' => [
                        'buckets' => [
                            ['id' => 'zeta', 'name' => 'Zeta', 'created_at' => '2026-09-06T00:00:03Z', 'updated_at' => '2026-09-06T00:00:03Z'],
                            ['id' => 'alpha-images', 'name' => 'Images', 'created_at' => '2026-09-06T00:00:01Z', 'updated_at' => '2026-09-06T00:00:02Z'],
                            ['id' => 'beta-images', 'name' => 'Images Archive', 'created_at' => '2026-09-06T00:00:02Z', 'updated_at' => '2026-09-06T00:00:04Z'],
                        ],
                    ],
                ],
            ]],
        ]);
        config(['studio.registry_path' => $path]);

        try {
            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/storage/alpha/buckets?search=images&sortColumn=id&sortOrder=asc&limit=1&offset=1')
                ->assertOk()
                ->assertJsonCount(1)
                ->assertJsonPath('0.id', 'beta-images');
        } finally {
            @unlink($path);
        }
    }

    public function test_bucket_listing_is_project_scoped_and_synthesizes_the_provisioned_bucket(): void
    {
        $path = $this->writeRegistry([
            'currentProjectId' => 'alpha',
            'projects' => [[
                'id' => 'alpha',
                'name' => 'Alpha',
                'status' => 'ready',
                'createdAt' => '2026-09-06T00:00:00Z',
                'scope' => ['storage' => ['bucket' => 'supadata-alpha']],
            ]],
        ]);
        config(['studio.registry_path' => $path]);

        try {
            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/storage/alpha/buckets')
                ->assertOk()
                ->assertJsonPath('0.id', 'supadata-alpha')
                ->assertJsonPath('0.name', 'supadata-alpha')
                ->assertJsonPath('0.public', false)
                ->assertJsonPath('0.type', 'STANDARD');

            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/storage/missing/buckets')
                ->assertNotFound();
        } finally {
            @unlink($path);
        }
    }

    public function test_bucket_listing_does_not_claim_a_bucket_for_failed_provisioning(): void
    {
        $path = $this->writeRegistry([
            'currentProjectId' => 'failed',
            'projects' => [[
                'id' => 'failed',
                'name' => 'Failed',
                'status' => 'failed',
                'scope' => ['storage' => ['bucket' => 'supadata-failed']],
            ]],
        ]);
        config(['studio.registry_path' => $path]);

        try {
            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/storage/failed/buckets')
                ->assertOk()
                ->assertExactJson([]);
        } finally {
            @unlink($path);
        }
    }

    public function test_bucket_creation_persists_project_scoped_metadata(): void
    {
        $path = $this->writeRegistry([
            'currentProjectId' => 'alpha',
            'projects' => [['id' => 'alpha', 'name' => 'Alpha', 'status' => 'ready', 'scope' => ['storage' => ['buckets' => []]]]],
        ]);
        config(['studio.registry_path' => $path]);

        try {
            $this->withBasicAuth('studio', 'password')
                ->postJson('/api/platform/storage/alpha/buckets', [
                    'id' => 'uploads',
                    'public' => false,
                    'type' => 'STANDARD',
                    'allowed_mime_types' => ['image/png'],
                    'file_size_limit' => 1048576,
                ])
                ->assertOk()
                ->assertExactJson(['name' => 'uploads']);

            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/storage/alpha/buckets')
                ->assertOk()
                ->assertJsonPath('0.id', 'uploads')
                ->assertJsonPath('0.allowed_mime_types.0', 'image/png')
                ->assertJsonPath('0.file_size_limit', 1048576);
        } finally {
            @unlink($path);
        }
    }

    public function test_bucket_creation_rejects_duplicates(): void
    {
        $path = $this->writeRegistry([
            'currentProjectId' => 'alpha',
            'projects' => [['id' => 'alpha', 'name' => 'Alpha', 'status' => 'ready', 'scope' => ['storage' => ['buckets' => [['id' => 'uploads']]]]]],
        ]);
        config(['studio.registry_path' => $path]);

        try {
            $this->withBasicAuth('studio', 'password')
                ->postJson('/api/platform/storage/alpha/buckets', ['id' => 'uploads'])
                ->assertStatus(409);
        } finally {
            @unlink($path);
        }
    }

    /** @param array<string, mixed> $registry */
    private function writeRegistry(array $registry): string
    {
        $path = tempnam(sys_get_temp_dir(), 'supadata-registry-');
        file_put_contents($path, json_encode($registry, JSON_THROW_ON_ERROR));
        return $path;
    }
}
