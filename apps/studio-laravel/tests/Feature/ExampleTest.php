<?php

namespace Tests\Feature;

// use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ExampleTest extends TestCase
{
    public function test_the_studio_shell_requires_authentication(): void
    {
        $this->get('/')->assertUnauthorized();
        $this->withBasicAuth('studio', 'password')->get('/')->assertOk();
    }

    public function test_internal_control_plane_bearer_auth_can_read_project_metadata(): void
    {
        config()->set('studio.control_plane_token', 'internal-test-token');

        $this->withHeader('Authorization', 'Bearer internal-test-token')
            ->getJson('/api/platform/projects/default')
            ->assertOk()
            ->assertJsonPath('ref', 'default');
    }
}
