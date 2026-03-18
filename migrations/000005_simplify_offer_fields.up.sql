BEGIN;

ALTER TABLE wholesaler_offers
    DROP CONSTRAINT IF EXISTS ck_wholesaler_offers_min_order_qty_positive;

UPDATE wholesaler_offers
SET min_order_qty = 1;

ALTER TABLE wholesaler_offers
    ALTER COLUMN min_order_qty SET DEFAULT 1;

ALTER TABLE wholesaler_offers
    ADD CONSTRAINT ck_wholesaler_offers_min_order_qty_one CHECK (min_order_qty = 1);

ALTER TABLE wholesaler_offers
    DROP COLUMN IF EXISTS delivery_eta_hours;

COMMIT;
