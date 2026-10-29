<?php

namespace Tests\Feature;

use Tests\TestCase;

class StudioSignInTest extends TestCase
{
    public function test_browser_guests_are_redirected_to_the_sign_in_page(): void
    {
        $this->get('/project/default')->assertRedirect('/sign-in?returnTo=%2Fproject%2Fdefault');
        $this->get('/sign-in')->assertOk();
        $this->getJson('/api/platform/projects')->assertUnauthorized();
    }

    public function test_deployment_credentials_create_a_session_and_logout_revokes_it(): void
    {
        $this->postJson('/api/studio-auth/login', ['username' => 'studio', 'password' => 'password'])
            ->assertOk()->assertJson(['authenticated' => true]);
        $this->getJson('/api/platform/projects')->assertOk();
        $this->postJson('/api/studio-auth/logout')->assertOk();
        $this->getJson('/api/platform/projects')->assertUnauthorized();
    }

    public function test_invalid_credentials_do_not_authenticate_a_session(): void
    {
        $this->postJson('/api/studio-auth/login', ['username' => 'studio', 'password' => 'wrong-password'])
            ->assertUnauthorized();
        $this->getJson('/api/platform/projects')->assertUnauthorized();
    }

    public function test_public_project_metadata_does_not_emit_localhost(): void
    {
        config(['app.url' => 'https://golara.craftmatrix.org']);
        $response = $this->withBasicAuth('studio', 'password')->getJson('/api/platform/projects/default');
        $response->assertOk();
        $this->assertStringNotContainsString('localhost', $response->getContent());
    }
}
