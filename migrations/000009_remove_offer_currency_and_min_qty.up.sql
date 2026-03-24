ALTER TABLE wholesaler_offers
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS min_order_qty;
