CREATE TYPE inventory_movement_type AS ENUM ('IN', 'OUT', 'RESERVED', 'RELEASED', 'ADJUST');

ALTER TABLE wholesaler_offers
    ADD COLUMN IF NOT EXISTS available_qty int NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS inventory_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    wholesaler_id uuid NOT NULL,
    offer_id uuid NOT NULL,
    type inventory_movement_type NOT NULL,
    qty int NOT NULL,
    ref_type text NULL,
    ref_id uuid NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_inventory_wh_offer_created
    ON inventory_movements (wholesaler_id, offer_id, created_at DESC);
