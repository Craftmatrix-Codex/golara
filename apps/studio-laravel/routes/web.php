<?php

use App\Http\Controllers\StudioController;
use Illuminate\Foundation\Http\Middleware\PreventRequestForgery;
use Illuminate\Support\Facades\Route;

Route::get('/health', [StudioController::class, 'health']);

Route::middleware('studio.auth')
    ->withoutMiddleware([PreventRequestForgery::class])
    ->group(function (): void {
    Route::get('/api/get-utc-time', [StudioController::class, 'utcTime']);
    Route::post('/api/telemetry', [StudioController::class, 'recordTelemetry']);
    Route::get('/api/platform/telemetry', [StudioController::class, 'telemetryEvents']);
    Route::get('/api/get-deployment-commit', [StudioController::class, 'deploymentCommit']);
    Route::get('/api/enabled-features-overrides', [StudioController::class, 'featureOverrides']);
    Route::get('/api/cli-release-version', [StudioController::class, 'cliReleaseVersion']);
    Route::get('/api/platform/projects', [StudioController::class, 'projects']);
    Route::get('/api/platform/profile', [StudioController::class, 'profile']);
    Route::get('/api/platform/organizations', [StudioController::class, 'organizations']);
    Route::get('/api/platform/deployment-mode', [StudioController::class, 'deploymentMode']);
    Route::get('/api/platform/integrations/github/authorization', [StudioController::class, 'githubAuthorization']);
    Route::get('/api/platform/projects/current', [StudioController::class, 'currentProject']);
    Route::get('/api/platform/projects/{project}', [StudioController::class, 'project']);
    Route::get('/api/platform/projects/{project}/credentials', [StudioController::class, 'credentials']);
    Route::post('/api/platform/projects/{project}/credentials/{type}/rotate', [StudioController::class, 'rotateCredential']);
    Route::get('/api/ai/sql/check-api-key', [StudioController::class, 'aiSqlCheckApiKey']);
    Route::post('/api/platform/projects/{project}/api/graphql', [StudioController::class, 'graphql']);
    Route::match(['get', 'head'], '/api/platform/projects/{project}/api/rest', [StudioController::class, 'apiRest']);
    Route::get('/api/platform/projects/{project}/config/secrets/update-status', [StudioController::class, 'jwtSecretUpdateStatus']);
    Route::match(['get', 'post'], '/api/platform/projects/{project}/config/postgrest', [StudioController::class, 'postgrestConfig']);
    Route::match(['get', 'post'], '/api/platform/projects/{project}/analytics/endpoints/{name}', [StudioController::class, 'analytics']);
    Route::get('/api/platform/projects/{project}/settings', [StudioController::class, 'settings']);
    Route::get('/api/platform/projects/{project}/databases', [StudioController::class, 'databases']);
    Route::post('/api/platform/projects/{project}/connection-password', [StudioController::class, 'revealDatabasePassword']);
    Route::post('/api/platform/projects/{project}/api-keys/temporary', [StudioController::class, 'temporaryApiKey']);
    Route::get('/api/v1/projects/{project}/api-keys', [StudioController::class, 'apiKeys']);
    Route::match(['get', 'post'], '/api/platform/projects/{project}/run-lints', [StudioController::class, 'runLints']);
    Route::post('/api/platform/pg-meta/{project}/query', [StudioController::class, 'pgMetaQuery']);
    Route::get('/api/platform/projects/{project}/content/count', [StudioController::class, 'contentCount']);
    Route::match(['get', 'post'], '/api/platform/projects/{project}/content/folders', [StudioController::class, 'contentFolders']);
    Route::get('/api/platform/projects/{project}/content/folders/{id}', [StudioController::class, 'contentFolder']);
    Route::get('/api/platform/projects/{project}/content/item/{id}', [StudioController::class, 'contentItem']);
    Route::match(['get', 'put', 'delete'], '/api/platform/projects/{project}/content', [StudioController::class, 'content']);
    Route::match(['get', 'post'], '/api/platform/storage/{project}/buckets', [StudioController::class, 'storageBuckets']);
    Route::get('/api/platform/storage/{project}/vector-buckets', [StudioController::class, 'vectorBuckets']);
    Route::get('/api/v1/projects/{project}/functions', [StudioController::class, 'functions']);
    Route::post('/api/v1/projects/{project}/functions/deploy', [StudioController::class, 'deployFunction']);
    Route::get('/api/v1/projects/{project}/functions/{slug}/body', [StudioController::class, 'functionBody']);
    Route::match(['get', 'patch'], '/api/v1/projects/{project}/functions/{slug}', [StudioController::class, 'function']);
    Route::delete('/api/v1/projects/{project}/functions/{slug}', [StudioController::class, 'deleteFunction']);
    Route::get('/api/platform/projects/{project}/analytics/log-drains', [StudioController::class, 'logDrains']);
    Route::any('/api/{path?}', [StudioController::class, 'apiNotFound'])
        ->where('path', '.*');

    Route::get('/{path?}', [StudioController::class, 'client'])
        ->where('path', '.*');
});
