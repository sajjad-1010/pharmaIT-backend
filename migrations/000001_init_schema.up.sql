BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TYPE user_role AS ENUM ('PHARMACY', 'WHOLESALER', 'MANUFACTURER', 'ADMIN');
CREATE TYPE user_status AS ENUM ('PENDING', 'ACTIVE', 'SUSPENDED');
CREATE TYPE order_status AS ENUM ('CREATED', 'CONFIRMED', 'PACKING', 'SHIPPED', 'DELIVERED', 'CANCELED');
CREATE TYPE rare_request_status AS ENUM ('OPEN', 'IN_REVIEW', 'SELECTED', 'CLOSED', 'CANCELED');
CREATE TYPE rare_bid_status AS ENUM ('SUBMITTED', 'ACCEPTED', 'REJECTED', 'WITHDRAWN');
CREATE TYPE manufacturer_request_status AS ENUM ('CREATED', 'SENT', 'QUOTED', 'APPROVED', 'REJECTED', 'CLOSED');
CREATE TYPE discount_campaign_status AS ENUM ('DRAFT', 'ACTIVE', 'PAUSED', 'ENDED');
CREATE TYPE discount_type AS ENUM ('PERCENT', 'FIXED');
CREATE TYPE payment_status AS ENUM ('PENDING', 'PAID', 'FAILED', 'REVERSED');
CREATE TYPE outbox_status AS ENUM ('NEW', 'PROCESSED', 'FAILED');
CREATE TYPE inventory_movement_type AS ENUM ('IN', 'OUT', 'RESERVED', 'RELEASED', 'ADJUST');

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text UNIQUE,
    phone text UNIQUE,
    password_hash text NOT NULL,
    role user_role NOT NULL,
    status user_status NOT NULL DEFAULT 'PENDING',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_users_identifier_present CHECK (email IS NOT NULL OR phone IS NOT NULL)
);

CREATE TABLE pharmacies (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    city text,
    address text,
    license_no text
);

CREATE TABLE wholesalers (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    country text,
    city text,
    address text,
    license_no text
);

CREATE TABLE manufacturers (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    country text,
    registration_no text
);

CREATE TABLE medicines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    manufacturer_id uuid NOT NULL REFERENCES manufacturers(user_id) ON DELETE RESTRICT,
    generic_name text NOT NULL,
    brand_name text,
    form text NOT NULL,
    strength text,
    pack_size text,
    atc_code text,
    is_active boolean NOT NULL DEFAULT TRUE,
    search_vector tsvector,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE wholesaler_offers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    wholesaler_id uuid NOT NULL REFERENCES wholesalers(user_id) ON DELETE CASCADE,
    medicine_id uuid NOT NULL REFERENCES medicines(id) ON DELETE RESTRICT,
    display_price numeric(18,4) NOT NULL,
    currency text NOT NULL,
    available_qty int NOT NULL DEFAULT 0,
    min_order_qty int NOT NULL DEFAULT 1,
    delivery_eta_hours int,
    expiry_date date,
    is_active boolean NOT NULL DEFAULT TRUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_wholesaler_medicine_active_offer UNIQUE (wholesaler_id, medicine_id, is_active),
    CONSTRAINT ck_wholesaler_offers_display_price_non_negative CHECK (display_price >= 0),
    CONSTRAINT ck_wholesaler_offers_available_qty_non_negative CHECK (available_qty >= 0),
    CONSTRAINT ck_wholesaler_offers_min_order_qty_positive CHECK (min_order_qty > 0)
);

CREATE TABLE orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pharmacy_id uuid NOT NULL REFERENCES pharmacies(user_id) ON DELETE RESTRICT,
    wholesaler_id uuid NOT NULL REFERENCES wholesalers(user_id) ON DELETE RESTRICT,
    status order_status NOT NULL DEFAULT 'CREATED',
    total_amount numeric(18,4) NOT NULL DEFAULT 0,
    currency text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_orders_total_amount_non_negative CHECK (total_amount >= 0)
);

CREATE TABLE order_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    medicine_id uuid NOT NULL REFERENCES medicines(id) ON DELETE RESTRICT,
    qty int NOT NULL,
    unit_price numeric(18,4) NOT NULL,
    line_total numeric(18,4) NOT NULL,
    CONSTRAINT ck_order_items_qty_positive CHECK (qty > 0),
    CONSTRAINT ck_order_items_unit_price_non_negative CHECK (unit_price >= 0),
    CONSTRAINT ck_order_items_line_total CHECK (line_total = round((qty::numeric * unit_price), 4))
);

CREATE TABLE rare_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pharmacy_id uuid NOT NULL REFERENCES pharmacies(user_id) ON DELETE RESTRICT,
    medicine_id uuid REFERENCES medicines(id) ON DELETE RESTRICT,
    requested_name_text text,
    qty int NOT NULL,
    deadline_at timestamptz NOT NULL,
    notes text,
    status rare_request_status NOT NULL DEFAULT 'OPEN',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_rare_requests_qty_positive CHECK (qty > 0),
    CONSTRAINT ck_rare_requests_medicine_or_name CHECK (medicine_id IS NOT NULL OR requested_name_text IS NOT NULL)
);

CREATE TABLE rare_bids (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rare_request_id uuid NOT NULL REFERENCES rare_requests(id) ON DELETE CASCADE,
    wholesaler_id uuid NOT NULL REFERENCES wholesalers(user_id) ON DELETE RESTRICT,
    price numeric(18,4) NOT NULL,
    currency text NOT NULL,
    available_qty int NOT NULL DEFAULT 0,
    delivery_eta_hours int,
    notes text,
    status rare_bid_status NOT NULL DEFAULT 'SUBMITTED',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_rare_bids_price_non_negative CHECK (price >= 0),
    CONSTRAINT ck_rare_bids_available_qty_non_negative CHECK (available_qty >= 0)
);

CREATE TABLE manufacturer_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    wholesaler_id uuid NOT NULL REFERENCES wholesalers(user_id) ON DELETE RESTRICT,
    manufacturer_id uuid NOT NULL REFERENCES manufacturers(user_id) ON DELETE RESTRICT,
    medicine_id uuid REFERENCES medicines(id) ON DELETE RESTRICT,
    requested_name_text text,
    qty int NOT NULL,
    needed_by timestamptz,
    status manufacturer_request_status NOT NULL DEFAULT 'CREATED',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_manufacturer_requests_qty_positive CHECK (qty > 0),
    CONSTRAINT ck_manufacturer_requests_medicine_or_name CHECK (medicine_id IS NOT NULL OR requested_name_text IS NOT NULL)
);

CREATE TABLE manufacturer_quotes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id uuid NOT NULL REFERENCES manufacturer_requests(id) ON DELETE CASCADE,
    manufacturer_id uuid NOT NULL REFERENCES manufacturers(user_id) ON DELETE RESTRICT,
    unit_price_final numeric(18,4) NOT NULL,
    currency text NOT NULL,
    lead_time_days int,
    valid_until timestamptz,
    notes text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_manufacturer_quotes_unit_price_non_negative CHECK (unit_price_final >= 0)
);

CREATE TABLE discount_campaigns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    wholesaler_id uuid NOT NULL REFERENCES wholesalers(user_id) ON DELETE CASCADE,
    title text NOT NULL,
    starts_at timestamptz,
    ends_at timestamptz,
    status discount_campaign_status NOT NULL DEFAULT 'DRAFT'
);

CREATE TABLE discount_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES discount_campaigns(id) ON DELETE CASCADE,
    medicine_id uuid NOT NULL REFERENCES medicines(id) ON DELETE RESTRICT,
    discount_type discount_type NOT NULL,
    discount_value numeric(18,4) NOT NULL
);

CREATE TABLE payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount numeric(18,4) NOT NULL,
    currency text NOT NULL,
    invoice_id text NOT NULL UNIQUE,
    transaction_id text UNIQUE,
    status payment_status NOT NULL DEFAULT 'PENDING',
    paid_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_payments_amount_non_negative CHECK (amount >= 0)
);

CREATE TABLE access_passes (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    access_until timestamptz NOT NULL
);

CREATE TABLE outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type text NOT NULL,
    payload_json jsonb NOT NULL,
    status outbox_status NOT NULL DEFAULT 'NEW',
    created_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz
);

CREATE TABLE inventory_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    wholesaler_id uuid NOT NULL REFERENCES wholesalers(user_id) ON DELETE RESTRICT,
    medicine_id uuid NOT NULL REFERENCES medicines(id) ON DELETE RESTRICT,
    type inventory_movement_type NOT NULL,
    qty int NOT NULL,
    ref_type text,
    ref_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_inventory_movements_qty_positive CHECK (qty > 0)
);

CREATE INDEX idx_medicines_manufacturer_id ON medicines (manufacturer_id);
CREATE INDEX idx_medicines_generic_name ON medicines (generic_name);
CREATE INDEX idx_medicines_brand_name ON medicines (brand_name);
CREATE INDEX idx_medicines_generic_trgm ON medicines USING gin (generic_name gin_trgm_ops);
CREATE INDEX idx_medicines_brand_trgm ON medicines USING gin (brand_name gin_trgm_ops);
CREATE INDEX idx_medicines_search_vector ON medicines USING gin (search_vector);

CREATE INDEX idx_wholesaler_offers_medicine_active_updated ON wholesaler_offers (medicine_id, is_active, updated_at DESC);
CREATE INDEX idx_wholesaler_offers_wholesaler_active ON wholesaler_offers (wholesaler_id, is_active);
CREATE INDEX idx_wholesaler_offers_cursor ON wholesaler_offers (medicine_id, is_active, updated_at DESC, id DESC);

CREATE INDEX idx_orders_pharmacy_created ON orders (pharmacy_id, created_at);
CREATE INDEX idx_orders_wholesaler_created ON orders (wholesaler_id, created_at);
CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_pharmacy_cursor ON orders (pharmacy_id, created_at DESC, id DESC);
CREATE INDEX idx_orders_wholesaler_cursor ON orders (wholesaler_id, created_at DESC, id DESC);

CREATE INDEX idx_rare_requests_status_deadline ON rare_requests (status, deadline_at);
CREATE INDEX idx_rare_requests_cursor ON rare_requests (status, deadline_at ASC, id ASC);
CREATE INDEX idx_rare_bids_request_status ON rare_bids (rare_request_id, status);

CREATE INDEX idx_manufacturer_requests_manufacturer_status ON manufacturer_requests (manufacturer_id, status);
CREATE INDEX idx_manufacturer_requests_wholesaler_status ON manufacturer_requests (wholesaler_id, status);
CREATE INDEX idx_manufacturer_requests_cursor ON manufacturer_requests (manufacturer_id, status, created_at DESC, id DESC);
CREATE INDEX idx_manufacturer_quotes_request ON manufacturer_quotes (request_id);

CREATE INDEX idx_payments_user_created ON payments (user_id, created_at);
CREATE INDEX idx_outbox_status_created ON outbox (status, created_at);
CREATE INDEX idx_inventory_wholesaler_medicine_created ON inventory_movements (wholesaler_id, medicine_id, created_at DESC);

CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_medicines_updated_at
    BEFORE UPDATE ON medicines
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_wholesaler_offers_updated_at
    BEFORE UPDATE ON wholesaler_offers
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_rare_requests_updated_at
    BEFORE UPDATE ON rare_requests
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_rare_bids_updated_at
    BEFORE UPDATE ON rare_bids
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_manufacturer_requests_updated_at
    BEFORE UPDATE ON manufacturer_requests
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE FUNCTION medicines_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('simple', coalesce(NEW.generic_name, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(NEW.brand_name, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(NEW.form, '')), 'C') ||
        setweight(to_tsvector('simple', coalesce(NEW.strength, '')), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_medicines_search_vector
    BEFORE INSERT OR UPDATE ON medicines
    FOR EACH ROW
    EXECUTE FUNCTION medicines_search_vector_update();

COMMIT;

