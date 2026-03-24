ALTER TABLE wholesaler_offers
    ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'TJS',
    ADD COLUMN IF NOT EXISTS min_order_qty int NOT NULL DEFAULT 1;
