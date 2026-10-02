<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        if (! Schema::hasTable('auth.users')) {
            Schema::create('auth.users', function (Blueprint $table): void {
                $table->uuid('id')->primary();
                $table->text('email')->nullable();
                $table->timestampTz('banned_until')->nullable();
                $table->timestampTz('created_at')->useCurrent();
                $table->timestampTz('confirmed_at')->nullable();
                $table->timestampTz('email_confirmed_at')->nullable();
                $table->timestampTz('confirmation_sent_at')->nullable();
                $table->boolean('is_anonymous')->default(false);
                $table->boolean('is_sso_user')->default(false);
                $table->timestampTz('invited_at')->nullable();
                $table->timestampTz('last_sign_in_at')->nullable();
                $table->text('phone')->nullable();
                $table->timestampTz('phone_confirmed_at')->nullable();
                $table->jsonb('raw_app_meta_data')->default('{}');
                $table->jsonb('raw_user_meta_data')->default('{}');
                $table->timestampTz('updated_at')->useCurrent();
            });
        } else {
            Schema::table('auth.users', function (Blueprint $table): void {
                if (! Schema::hasColumn('auth.users', 'email_confirmed_at')) {
                    $table->timestampTz('email_confirmed_at')->nullable();
                }
                if (! Schema::hasColumn('auth.users', 'phone_confirmed_at')) {
                    $table->timestampTz('phone_confirmed_at')->nullable();
                }
                if (! Schema::hasColumn('auth.users', 'confirmed_at')) {
                    $table->timestampTz('confirmed_at')->nullable();
                }
            });
        }

        if (! Schema::hasTable('auth.identities')) {
            Schema::create('auth.identities', function (Blueprint $table): void {
                $table->uuid('id')->primary();
                $table->uuid('user_id');
                $table->text('provider');
                $table->jsonb('identity_data')->default('{}');
                $table->timestampTz('created_at')->useCurrent();
                $table->timestampTz('updated_at')->useCurrent();
                $table->foreign('user_id')->references('id')->on('auth.users')->cascadeOnDelete();
                $table->index('user_id', 'identities_user_id_idx');
            });
        }
    }

    public function down(): void
    {
        Schema::dropIfExists('auth.identities');
        Schema::dropIfExists('auth.users');
    }
};
