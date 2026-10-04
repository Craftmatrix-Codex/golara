<?php

namespace Tests\Feature;

use Illuminate\Support\Facades\File;
use Illuminate\Support\Str;
use Illuminate\Http\UploadedFile;
use Tests\TestCase;

class StudioEdgeFunctionsTest extends TestCase
{
    private string $functionsPath;

    protected function setUp(): void
    {
        parent::setUp();
        $this->functionsPath = sys_get_temp_dir() . '/supadata-functions-' . Str::uuid()->toString();
        config(['studio.functions_path' => $this->functionsPath]);
    }

    protected function tearDown(): void
    {
        File::deleteDirectory($this->functionsPath);
        parent::tearDown();
    }

    public function test_function_deploy_list_detail_body_update_and_delete_contracts(): void
    {
        $auth = $this->withBasicAuth('studio', 'password');
        $metadata = json_encode([
            'entrypoint_path' => 'index.ts',
            'verify_jwt' => false,
        ], JSON_THROW_ON_ERROR);

        $auth->post('/api/v1/projects/default/functions/deploy?slug=hello', [
            'metadata' => $metadata,
            'file' => [UploadedFile::fake()->createWithContent('index.ts', "Deno.serve(() => Response.json({ ok: true }))\n")],
        ])
            ->assertCreated()
            ->assertJsonPath('slug', 'hello')
            ->assertJsonPath('status', 'ACTIVE')
            ->assertJsonPath('version', 1);

        $auth->getJson('/api/v1/projects/default/functions')
            ->assertOk()
            ->assertJsonCount(1)
            ->assertJsonPath('0.slug', 'hello');

        $auth->getJson('/api/v1/projects/default/functions/hello')
            ->assertOk()
            ->assertJsonPath('slug', 'hello')
            ->assertJsonPath('verify_jwt', false);

        $body = $auth->get('/api/v1/projects/default/functions/hello/body', [
            'Accept' => 'multipart/form-data',
        ])->assertOk();
        $body->assertHeader('Content-Type');
        $this->assertStringContainsString('multipart/form-data; boundary=', (string) $body->headers->get('Content-Type'));
        $this->assertStringContainsString('filename="index.ts"', $body->getContent());
        $this->assertStringContainsString('Deno.serve', $body->getContent());

        $auth->patchJson('/api/v1/projects/default/functions/hello', [
            'name' => 'Hello world',
            'verify_jwt' => true,
        ])->assertOk()
            ->assertJsonPath('name', 'Hello world')
            ->assertJsonPath('verify_jwt', true);

        $auth->deleteJson('/api/v1/projects/default/functions/hello')
            ->assertOk()
            ->assertJsonPath('message', 'ok');

        $auth->getJson('/api/v1/projects/default/functions')->assertOk()->assertJsonCount(0);
    }

    public function test_function_deploy_rejects_unsafe_paths(): void
    {
        $metadata = json_encode(['entrypoint_path' => '../secret.ts'], JSON_THROW_ON_ERROR);
        $this->withBasicAuth('studio', 'password')
            ->post('/api/v1/projects/default/functions/deploy?slug=unsafe', [
                'metadata' => $metadata,
                'file' => [UploadedFile::fake()->createWithContent('../secret.ts', 'unsafe')],
            ])
            ->assertUnprocessable();
    }
}
