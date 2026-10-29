<?php

namespace Tests;

use Illuminate\Foundation\Testing\TestCase as BaseTestCase;
use Illuminate\Support\Facades\File;

abstract class TestCase extends BaseTestCase
{
    private string $fixtureDirectory;

    protected function setUp(): void
    {
        parent::setUp();
        $this->fixtureDirectory = sys_get_temp_dir() . '/golara-laravel-tests-' . bin2hex(random_bytes(8));
        mkdir($this->fixtureDirectory, 0700, true);
        // This fixture exercises the Laravel shell boundary; Playwright verifies the built React app.
        file_put_contents($this->fixtureDirectory . '/index.html', '<!doctype html><html><head><title>Supabase</title></head><body><div id="studio-shell-loader"></div></body></html>');
        config([
            'app.key' => 'base64:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=',
            'studio.client_shell' => $this->fixtureDirectory . '/index.html',
            'studio.registry_path' => $this->fixtureDirectory . '/registry.json',
            'studio.content_path' => $this->fixtureDirectory . '/content',
            'studio.functions_path' => $this->fixtureDirectory . '/functions',
        ]);
    }

    protected function tearDown(): void
    {
        File::deleteDirectory($this->fixtureDirectory);
        parent::tearDown();
    }
}
