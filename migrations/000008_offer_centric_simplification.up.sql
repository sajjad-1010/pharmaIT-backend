DROP TRIGGER IF EXISTS trg_medicines_search_vector ON medicines;
DROP FUNCTION IF EXISTS medicines_search_vector_update();

DROP INDEX IF EXISTS idx_medicines_generic_trgm;
DROP INDEX IF EXISTS idx_medicines_brand_trgm;
DROP INDEX IF EXISTS idx_medicines_search_vector;
DROP INDEX IF EXISTS idx_medicines_generic_name;
DROP INDEX IF EXISTS idx_medicines_brand_name;

DROP INDEX IF EXISTS idx_wholesaler_offers_medicine_active_updated;
DROP INDEX IF EXISTS idx_wholesaler_offers_cursor;
DROP INDEX IF EXISTS idx_inventory_wholesaler_medicine_created;

ALTER TABLE wholesaler_offers
    DROP CONSTRAINT IF EXISTS wholesaler_offers_medicine_id_fkey,
    DROP CONSTRAINT IF EXISTS uq_wholesaler_medicine_active_offer;

ALTER TABLE order_items
    DROP CONSTRAINT IF EXISTS order_items_medicine_id_fkey;

ALTER TABLE rare_requests
    DROP CONSTRAINT IF EXISTS rare_requests_medicine_id_fkey,
    DROP CONSTRAINT IF EXISTS ck_rare_requests_medicine_or_name;

ALTER TABLE manufacturer_requests
    DROP CONSTRAINT IF EXISTS manufacturer_requests_medicine_id_fkey,
    DROP CONSTRAINT IF EXISTS ck_manufacturer_requests_medicine_or_name;

ALTER TABLE discount_items
    DROP CONSTRAINT IF EXISTS discount_items_medicine_id_fkey;

ALTER TABLE inventory_movements
    DROP CONSTRAINT IF EXISTS inventory_movements_medicine_id_fkey;

ALTER TABLE wholesaler_offers
    ADD COLUMN name text,
    ADD COLUMN producer text;

UPDATE wholesaler_offers
SET name = COALESCE(name, '');

ALTER TABLE wholesaler_offers
    ALTER COLUMN name SET NOT NULL,
    DROP COLUMN IF EXISTS medicine_id;

ALTER TABLE order_items
    ADD COLUMN offer_id uuid,
    ADD COLUMN item_name text,
    ADD COLUMN producer text;

UPDATE order_items
SET offer_id = NULL,
    item_name = COALESCE(item_name, '');

ALTER TABLE order_items
    ALTER COLUMN offer_id SET NOT NULL,
    ALTER COLUMN item_name SET NOT NULL,
    DROP COLUMN IF EXISTS medicine_id;

ALTER TABLE order_items
    ADD CONSTRAINT order_items_offer_id_fkey
        FOREIGN KEY (offer_id) REFERENCES wholesaler_offers(id) ON DELETE RESTRICT;

ALTER TABLE inventory_movements
    ADD COLUMN offer_id uuid;

ALTER TABLE inventory_movements
    ALTER COLUMN offer_id SET NOT NULL,
    DROP COLUMN IF EXISTS medicine_id;

ALTER TABLE inventory_movements
    ADD CONSTRAINT inventory_movements_offer_id_fkey
        FOREIGN KEY (offer_id) REFERENCES wholesaler_offers(id) ON DELETE CASCADE;

ALTER TABLE rare_requests
    ALTER COLUMN requested_name_text SET NOT NULL,
    DROP COLUMN IF EXISTS medicine_id;

ALTER TABLE manufacturer_requests
    ALTER COLUMN requested_name_text SET NOT NULL,
    DROP COLUMN IF EXISTS medicine_id;

ALTER TABLE discount_items
    ADD COLUMN offer_id uuid;

ALTER TABLE discount_items
    ALTER COLUMN offer_id SET NOT NULL,
    DROP COLUMN IF EXISTS medicine_id;

ALTER TABLE discount_items
    ADD CONSTRAINT discount_items_offer_id_fkey
        FOREIGN KEY (offer_id) REFERENCES wholesaler_offers(id) ON DELETE CASCADE;

DROP TABLE IF EXISTS medicine_candidates;
DROP TABLE IF EXISTS medicines;

CREATE INDEX idx_wholesaler_offers_name_updated ON wholesaler_offers (updated_at DESC, id DESC);
CREATE INDEX idx_wholesaler_offers_name_trgm ON wholesaler_offers USING gin (name gin_trgm_ops);
CREATE INDEX idx_inventory_wholesaler_offer_created ON inventory_movements (wholesaler_id, offer_id, created_at DESC);
