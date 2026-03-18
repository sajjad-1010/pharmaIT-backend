BEGIN;

ALTER TABLE medicines
    ADD COLUMN IF NOT EXISTS manufacturer_id uuid;

ALTER TABLE medicines
    ADD CONSTRAINT medicines_manufacturer_id_fkey
    FOREIGN KEY (manufacturer_id) REFERENCES manufacturers(user_id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_medicines_manufacturer_id ON medicines (manufacturer_id);

COMMIT;
