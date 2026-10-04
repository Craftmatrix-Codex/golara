package jobs

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaSQL = `
CREATE SCHEMA IF NOT EXISTS cron;
CREATE TABLE IF NOT EXISTS cron.job (
  jobid bigserial PRIMARY KEY,
  schedule text NOT NULL,
  command text NOT NULL,
  nodename text NOT NULL DEFAULT 'localhost',
  nodeport integer NOT NULL DEFAULT 5432,
  database text NOT NULL DEFAULT current_database(),
  username text NOT NULL DEFAULT current_user,
  active boolean NOT NULL DEFAULT true,
  jobname text UNIQUE,
  last_scheduled_at timestamptz
);
CREATE TABLE IF NOT EXISTS cron.job_run_details (
  runid bigserial PRIMARY KEY,
  jobid bigint NOT NULL,
  job_pid integer,
  database text,
  username text,
  command text,
  status text,
  return_message text,
  start_time timestamptz,
  end_time timestamptz
);
CREATE OR REPLACE FUNCTION cron.schedule(job_name text, schedule text, command text)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE result bigint;
BEGIN
  INSERT INTO cron.job (jobname, schedule, command)
  VALUES (job_name, schedule, command)
  ON CONFLICT (jobname) DO UPDATE SET schedule = EXCLUDED.schedule, command = EXCLUDED.command, active = true
  RETURNING jobid INTO result;
  RETURN result;
END;
$$;
CREATE OR REPLACE FUNCTION cron.schedule(schedule text, command text)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE result bigint;
BEGIN
  INSERT INTO cron.job (jobname, schedule, command)
  VALUES ('job-' || gen_random_uuid()::text, schedule, command)
  RETURNING jobid INTO result;
  RETURN result;
END;
$$;
CREATE OR REPLACE FUNCTION cron.unschedule(target_jobid bigint)
RETURNS boolean
LANGUAGE plpgsql
AS $$
BEGIN
  DELETE FROM cron.job WHERE jobid = target_jobid;
  RETURN FOUND;
END;
$$;
CREATE OR REPLACE FUNCTION cron.unschedule(target_jobname text)
RETURNS boolean
LANGUAGE plpgsql
AS $$
BEGIN
  DELETE FROM cron.job WHERE jobname = target_jobname;
  RETURN FOUND;
END;
$$;
CREATE OR REPLACE FUNCTION cron.alter_job(
  job_id bigint,
  schedule text DEFAULT NULL,
  command text DEFAULT NULL,
  database text DEFAULT NULL,
  username text DEFAULT NULL,
  active boolean DEFAULT NULL
)
RETURNS void
LANGUAGE sql
AS $$
  UPDATE cron.job SET
    schedule = COALESCE(alter_job.schedule, cron.job.schedule),
    command = COALESCE(alter_job.command, cron.job.command),
    database = COALESCE(alter_job.database, cron.job.database),
    username = COALESCE(alter_job.username, cron.job.username),
    active = COALESCE(alter_job.active, cron.job.active)
  WHERE jobid = job_id
$$;`

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("reconcile jobs schema: %w", err)
	}
	return nil
}
