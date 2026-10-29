<?php

namespace App\Http\Middleware;

use App\Services\StudioSessionRegistry;
use Closure;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

class StudioBasicAuth
{
    public function __construct(private StudioSessionRegistry $sessions) {}

    public function handle(Request $request, Closure $next): Response
    {
        $username = (string) config('studio.auth_username');
        $password = (string) config('studio.auth_password');
        $providedUsername = (string) $request->getUser();
        $providedPassword = (string) $request->getPassword();
        $providedBearer = (string) $request->bearerToken();
        $controlPlaneToken = (string) config('studio.control_plane_token');

        if ($controlPlaneToken !== '' && hash_equals($controlPlaneToken, $providedBearer)) {
            return $next($request);
        }

        if ($request->session()->get('studio_authenticated') === true && $this->sessions->active($request->session()->getId())) {
            return $next($request);
        }

        $valid = $username !== ''
            && $password !== ''
            && hash_equals($username, $providedUsername)
            && hash_equals($password, $providedPassword);

        if (! $valid) {
            if ($request->expectsJson() || $request->is('api/*')) {
                return response()->json(['message' => 'Authentication required'], 401);
            }

            $returnTo = $request->getRequestUri();

            return redirect('/sign-in?returnTo='.rawurlencode($returnTo));
        }

        return $next($request);
    }
}
