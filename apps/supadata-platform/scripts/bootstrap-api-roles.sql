-- Run as the database administrator during GoLara installation.
-- service_role is a trusted server role and intentionally bypasses RLS.
-- Keep its API key out of browser clients.
BEGIN;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
    CREATE ROLE anon NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
    CREATE ROLE authenticated NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'service_role') THEN
    CREATE ROLE service_role NOLOGIN;
  END IF;
END;
$$;
ALTER ROLE anon NOBYPASSRLS;
ALTER ROLE authenticated NOBYPASSRLS;
ALTER ROLE service_role BYPASSRLS;
COMMIT;
