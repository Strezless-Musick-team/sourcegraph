BEGIN;

ALTER TABLE IF EXISTS event_logs DROP COLUMN IF EXISTS public_event_properties;

COMMIT;
