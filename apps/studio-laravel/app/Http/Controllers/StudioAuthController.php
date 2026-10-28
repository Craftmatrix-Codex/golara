<?php

namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class StudioAuthController
{
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

        $request->session()->regenerate();
        $request->session()->put('studio_authenticated', true);

        return response()->json(['authenticated' => true]);
    }

    public function logout(Request $request): JsonResponse
    {
        $request->session()->invalidate();
        $request->session()->regenerateToken();

        return response()->json(['authenticated' => false]);
    }
}
