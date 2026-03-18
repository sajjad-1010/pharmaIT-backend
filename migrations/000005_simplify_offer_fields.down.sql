BEGIN;

ALTER TABLE wholesaler_offers
    ADD COLUMN IF NOT EXISTS delivery_eta_hours int;

ALTER TABLE wholesaler_offers
    DROP CONSTRAINT IF EXISTS ck_wholesaler_offers_min_order_qty_one;

ALTER TABLE wholesaler_offers
    ADD CONSTRAINT ck_wholesaler_offers_min_order_qty_positive CHECK (min_order_qty > 0);

COMMIT;
