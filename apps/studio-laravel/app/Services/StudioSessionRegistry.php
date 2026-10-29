<?php

namespace App\Services;

use Illuminate\Support\Facades\Cache;

class StudioSessionRegistry
{
    public function activate(string $id): bool
    {
        return Cache::put($this->key($id), true, now()->addMinutes(max(1, (int) config('session.lifetime', 120))));
    }

    public function active(string $id): bool
    {
        return Cache::get($this->key($id)) === true;
    }

    public function revoke(string $id): bool
    {
        Cache::forget($this->key($id));

        return ! Cache::has($this->key($id));
    }

    private function key(string $id): string
    {
        return 'studio:active-session:'.hash('sha256', $id);
    }
}
