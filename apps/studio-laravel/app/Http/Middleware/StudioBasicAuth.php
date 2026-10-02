<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

class StudioBasicAuth
{
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

        $valid = $username !== ''
            && $password !== ''
            && hash_equals($username, $providedUsername)
            && hash_equals($password, $providedPassword);

        if (! $valid) {
            return response('Authentication required', 401, [
                'WWW-Authenticate' => 'Basic realm="Supadata Studio", charset="UTF-8"',
            ]);
        }

        return $next($request);
    }
}
