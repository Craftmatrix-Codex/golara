<?php

namespace App\Http\Controllers;

use App\Services\StudioSessionRegistry;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class StudioAuthController
{
    public function __construct(private StudioSessionRegistry $sessions) {}

    public function login(Request $request): JsonResponse
    {
        $credentials = $request->validate([
            'username' => ['required', 'string', 'max:255'],
            'password' => ['required', 'string', 'max:1024'],
        ]);

        $username = (string) config('studio.auth_username');
        $password = (string) config('studio.auth_password');
        $valid = $username !== ''
            && $password !== ''
            && hash_equals($username, $credentials['username'])
            && hash_equals($password, $credentials['password']);

        if (! $valid) {
            return response()->json(['message' => 'Invalid username or password.'], 401);
        }

        abort_unless($this->sessions->revoke($request->session()->getId()), 503, 'Unable to replace session.');
        $request->session()->regenerate(true);
        abort_unless($this->sessions->activate($request->session()->getId()), 503, 'Unable to create session.');
        $request->session()->put('studio_authenticated', true);

        return response()->json(['authenticated' => true]);
    }

    public function logout(Request $request): JsonResponse
    {
        abort_unless($this->sessions->revoke($request->session()->getId()), 503, 'Unable to end session.');
        $request->session()->invalidate();
        $request->session()->regenerateToken();

        return response()->json(['authenticated' => false]);
    }
}
