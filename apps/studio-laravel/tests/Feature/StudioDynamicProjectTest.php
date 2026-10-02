<?php

namespace Tests\Feature;

use Tests\TestCase;

class StudioDynamicProjectTest extends TestCase
{
    public function test_project_settings_never_falls_back_to_localhost_for_the_project_endpoint(): void
    {
        $path = tempnam(sys_get_temp_dir(), 'supadata-registry-');
        file_put_contents($path, json_encode([
            'currentProjectId' => 'default',
            'projects' => [[
                'id' => 'default',
                'name' => 'Default Project',
                'status' => 'ready',
                'scope' => ['publicUrl' => 'https://default.go-alpha.craftmatrix.org'],
            ]],
        ], JSON_THROW_ON_ERROR));
        config([
            'studio.registry_path' => $path,
            'studio.project_endpoint' => 'http://localhost',
            'app.url' => 'https://go-alpha.craftmatrix.org',
        ]);

        try {
            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/projects/default/settings')
                ->assertOk()
                ->assertJsonPath('app_config.protocol', 'https')
                ->assertJsonPath('app_config.endpoint', 'go-alpha.craftmatrix.org/api/projects/default')
                ->assertJsonPath('app_config.storage_endpoint', 'go-alpha.craftmatrix.org/api/projects/default')
                ->assertJsonMissing(['app_config.endpoint' => 'localhost']);
        } finally {
            @unlink($path);
        }
    }

    public function test_native_studio_contracts_read_projects_from_the_persisted_registry(): void
    {
        $path = tempnam(sys_get_temp_dir(), 'supadata-registry-');
        file_put_contents($path, json_encode([
            'currentProjectId' => 'video-project',
            'projects' => [[
                'id' => 'video-project',
                'name' => 'Video Project',
                'status' => 'ready',
                'current' => true,
                'createdAt' => '2026-09-06T00:00:00Z',
                'scope' => [
                    'database' => ['name' => 'supadata_video_project', 'role' => 'supadata_video_project_runtime', 'schema' => 'public'],
                    'storage' => ['bucket' => 'supadata-video-project'],
                    'publicUrl' => 'https://video-project.supabase.example.com',
                ],
            ]],
        ], JSON_THROW_ON_ERROR));
        config(['studio.registry_path' => $path]);

        try {
            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/projects')
                ->assertOk()
                ->assertJsonPath('0.ref', 'video-project')
                ->assertJsonPath('0.name', 'Video Project')
                ->assertJsonPath('0.status', 'ACTIVE_HEALTHY');

            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/projects/video-project')
                ->assertOk()
                ->assertJsonPath('ref', 'video-project')
                ->assertJsonPath('name', 'Video Project');

            $this->withBasicAuth('studio', 'password')
                ->getJson('/api/platform/projects/video-project/settings')
                ->assertOk()
                ->assertJsonPath('ref', 'video-project')
                ->assertJsonPath('name', 'Video Project')
                ->assertJsonPath('app_config.protocol', 'https')
                ->assertJsonPath('app_config.endpoint', 'video-project.supabase.example.com/api/projects/video-project')
                ->assertJsonPath('app_config.storage_endpoint', 'video-project.supabase.example.com/api/projects/video-project');
        } finally {
            @unlink($path);
        }
    }
}
