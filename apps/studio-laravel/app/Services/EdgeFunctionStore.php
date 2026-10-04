<?php

namespace App\Services;

use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Str;
use InvalidArgumentException;
use RuntimeException;

class EdgeFunctionStore
{
    private string $root;

    public function __construct()
    {
        $configured = config('studio.functions_path');
        $this->root = is_string($configured) && $configured !== ''
            ? $configured
            : '/var/lib/supadata/functions';
    }

    public function list(string $project): array
    {
        $projectDirectory = $this->projectDirectory($project);
        if (! is_dir($projectDirectory)) {
            return [];
        }

        $functions = [];
        foreach (File::directories($projectDirectory) as $directory) {
            if (str_starts_with(basename($directory), '.')) {
                continue;
            }
            try {
                $functions[] = $this->readManifest($directory);
            } catch (RuntimeException) {
                continue;
            }
        }
        usort($functions, fn (array $left, array $right): int => strcmp($left['slug'], $right['slug']));

        return $functions;
    }

    public function get(string $project, string $slug): array
    {
        return $this->readManifest($this->functionDirectory($project, $slug));
    }

    public function deploy(string $project, string $slug, array $metadata, array $files): array
    {
        $this->assertIdentifier($project, 'project');
        $this->assertIdentifier($slug, 'function slug');
        if ($files === []) {
            throw new InvalidArgumentException('At least one function file is required.');
        }

        $target = $this->functionDirectory($project, $slug);
        $existing = is_file($target . '/manifest.json') ? $this->readManifest($target) : null;
        $entrypoint = $this->safeRelativePath((string) ($metadata['entrypoint_path'] ?? 'index.ts'));
        $normalizedFiles = [];
        foreach ($files as $file) {
            if (! $file instanceof UploadedFile || ! $file->isValid()) {
                throw new InvalidArgumentException('Invalid function file upload.');
            }
            $name = $this->safeRelativePath($file->getClientOriginalName());
            $normalizedFiles[$name] = $file;
        }
        if (! array_key_exists($entrypoint, $normalizedFiles)) {
            throw new InvalidArgumentException('Function entrypoint was not uploaded.');
        }

        $now = now()->getTimestampMs();
        $manifest = [
            'id' => $existing['id'] ?? Str::uuid()->toString(),
            'name' => trim((string) ($metadata['name'] ?? $existing['name'] ?? $slug)) ?: $slug,
            'slug' => $slug,
            'status' => 'ACTIVE',
            'version' => (int) ($existing['version'] ?? 0) + 1,
            'created_at' => (int) ($existing['created_at'] ?? $now),
            'updated_at' => $now,
            'entrypoint_path' => $entrypoint,
            'import_map' => isset($metadata['import_map_path']),
            'import_map_path' => isset($metadata['import_map_path'])
                ? $this->safeRelativePath((string) $metadata['import_map_path'])
                : null,
            'verify_jwt' => (bool) ($metadata['verify_jwt'] ?? true),
            'files' => array_keys($normalizedFiles),
        ];

        $projectDirectory = $this->projectDirectory($project);
        File::ensureDirectoryExists($projectDirectory, 0750, true);
        $temporary = $projectDirectory . '/.' . $slug . '.tmp-' . Str::random(12);
        File::ensureDirectoryExists($temporary, 0750, true);
        try {
            foreach ($normalizedFiles as $name => $file) {
                $destination = $temporary . '/' . $name;
                File::ensureDirectoryExists(dirname($destination), 0750, true);
                $file->move(dirname($destination), basename($destination));
            }
            File::put($temporary . '/manifest.json', json_encode($manifest, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR) . "\n");
            if (is_dir($target)) {
                File::deleteDirectory($target);
            }
            if (! rename($temporary, $target)) {
                throw new RuntimeException('Could not activate function deployment.');
            }
        } finally {
            if (is_dir($temporary)) {
                File::deleteDirectory($temporary);
            }
        }

        return $manifest;
    }

    public function update(string $project, string $slug, array $changes): array
    {
        $directory = $this->functionDirectory($project, $slug);
        $manifest = $this->readManifest($directory);
        if (array_key_exists('name', $changes)) {
            $name = trim((string) $changes['name']);
            if ($name === '') {
                throw new InvalidArgumentException('Function name cannot be empty.');
            }
            $manifest['name'] = $name;
        }
        if (array_key_exists('verify_jwt', $changes)) {
            $manifest['verify_jwt'] = (bool) $changes['verify_jwt'];
        }
        $manifest['updated_at'] = now()->getTimestampMs();
        File::put($directory . '/manifest.json', json_encode($manifest, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR) . "\n");

        return $manifest;
    }

    public function delete(string $project, string $slug): void
    {
        $directory = $this->functionDirectory($project, $slug);
        if (! is_dir($directory)) {
            throw new RuntimeException('Function not found.');
        }
        File::deleteDirectory($directory);
    }

    public function files(string $project, string $slug): array
    {
        $directory = $this->functionDirectory($project, $slug);
        $manifest = $this->readManifest($directory);
        $files = [];
        foreach ($manifest['files'] ?? [] as $name) {
            if (! is_string($name)) {
                continue;
            }
            $safeName = $this->safeRelativePath($name);
            $path = $directory . '/' . $safeName;
            if (is_file($path)) {
                $files[$safeName] = (string) File::get($path);
            }
        }

        return ['metadata' => $manifest, 'files' => $files];
    }

    private function readManifest(string $directory): array
    {
        $path = $directory . '/manifest.json';
        if (! is_file($path)) {
            throw new RuntimeException('Function not found.');
        }
        $decoded = json_decode((string) File::get($path), true);
        if (! is_array($decoded) || ! is_string($decoded['slug'] ?? null)) {
            throw new RuntimeException('Invalid function manifest.');
        }

        return $decoded;
    }

    private function projectDirectory(string $project): string
    {
        $this->assertIdentifier($project, 'project');
        return $this->root . '/' . $project;
    }

    private function functionDirectory(string $project, string $slug): string
    {
        $this->assertIdentifier($slug, 'function slug');
        return $this->projectDirectory($project) . '/' . $slug;
    }

    private function assertIdentifier(string $value, string $label): void
    {
        if (preg_match('/^[a-z0-9](?:[a-z0-9_-]{0,62}[a-z0-9])?$/', $value) !== 1) {
            throw new InvalidArgumentException("Invalid {$label}.");
        }
    }

    private function safeRelativePath(string $path): string
    {
        $path = str_replace('\\', '/', trim($path));
        if ($path === '' || str_starts_with($path, '/') || str_contains($path, "\0")) {
            throw new InvalidArgumentException('Invalid function file path.');
        }
        $segments = explode('/', $path);
        if (in_array('..', $segments, true) || in_array('.', $segments, true) || in_array('', $segments, true)) {
            throw new InvalidArgumentException('Invalid function file path.');
        }

        return implode('/', $segments);
    }
}
