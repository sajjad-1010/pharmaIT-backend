BEGIN;

DROP TRIGGER IF EXISTS trg_medicines_search_vector ON medicines;
DROP FUNCTION IF EXISTS medicines_search_vector_update();

DROP TRIGGER IF EXISTS trg_manufacturer_requests_updated_at ON manufacturer_requests;
DROP TRIGGER IF EXISTS trg_rare_bids_updated_at ON rare_bids;
DROP TRIGGER IF EXISTS trg_rare_requests_updated_at ON rare_requests;
DROP TRIGGER IF EXISTS trg_orders_updated_at ON orders;
DROP TRIGGER IF EXISTS trg_wholesaler_offers_updated_at ON wholesaler_offers;
DROP TRIGGER IF EXISTS trg_medicines_updated_at ON medicines;
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS inventory_movements;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS access_passes;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS discount_items;
DROP TABLE IF EXISTS discount_campaigns;
DROP TABLE IF EXISTS manufacturer_quotes;
DROP TABLE IF EXISTS manufacturer_requests;
DROP TABLE IF EXISTS rare_bids;
DROP TABLE IF EXISTS rare_requests;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS wholesaler_offers;
DROP TABLE IF EXISTS medicines;
DROP TABLE IF EXISTS manufacturers;
DROP TABLE IF EXISTS wholesalers;
DROP TABLE IF EXISTS pharmacies;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS inventory_movement_type;
DROP TYPE IF EXISTS outbox_status;
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS discount_type;
DROP TYPE IF EXISTS discount_campaign_status;
DROP TYPE IF EXISTS manufacturer_request_status;
DROP TYPE IF EXISTS rare_bid_status;
DROP TYPE IF EXISTS rare_request_status;
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS user_status;
DROP TYPE IF EXISTS user_role;

COMMIT;

