-- Run as a superuser (e.g. postgres) via psql.

-- 1) Create role/user if it doesn't exist
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'hermod') THEN
CREATE ROLE hermod LOGIN PASSWORD 'hermod_password';
ELSE
    -- If user exists, ensure password is set/updated
    ALTER ROLE hermod WITH LOGIN PASSWORD 'hermod_password';
END IF;
END
$$;

-- 2) Create database if it doesn't exist
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'hermodts') THEN
    CREATE DATABASE hermodts OWNER hermod;
END IF;
END
$$;

-- 3) Ensure privileges (harmless if already correct)
GRANT ALL PRIVILEGES ON DATABASE hermodts TO hermod;

-- 4) Enable TimescaleDB extension inside hermodts
\connect hermodts

CREATE EXTENSION IF NOT EXISTS timescaledb;