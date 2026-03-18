BEGIN;

DROP INDEX IF EXISTS idx_medicines_manufacturer_id;

ALTER TABLE medicines
    DROP CONSTRAINT IF EXISTS medicines_manufacturer_id_fkey;

ALTER TABLE medicines
    DROP COLUMN IF EXISTS manufacturer_id;

COMMIT;
