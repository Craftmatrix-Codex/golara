<?php

namespace App\Services;

use Illuminate\Contracts\Cache\Repository;
use Illuminate\Support\Facades\Cache;

class StudioSessionRegistry
{
    public function activate(string $id): bool
    {
        return $this->cache()->put($this->key($id), true, now()->addMinutes(max(1, (int) config('session.lifetime', 120))));
    }

    public function active(string $id): bool
    {
        return $this->cache()->get($this->key($id)) === true;
    }

    public function revoke(string $id): bool
    {
        $this->cache()->forget($this->key($id));

        return ! $this->cache()->has($this->key($id));
    }

    private function cache(): Repository
    {
        return Cache::store((string) config('studio.session_cache_store', 'file'));
    }

    private function key(string $id): string
    {
        return 'studio:active-session:'.hash('sha256', $id);
    }
}
