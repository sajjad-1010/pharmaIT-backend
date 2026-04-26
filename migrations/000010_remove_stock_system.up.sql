ALTER TABLE wholesaler_offers
    DROP COLUMN IF EXISTS available_qty;

DROP TABLE IF EXISTS inventory_movements;

DROP TYPE IF EXISTS inventory_movement_type;
