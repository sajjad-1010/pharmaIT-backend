BEGIN;

CREATE TYPE notification_kind AS ENUM (
    'ORDER_CREATED',
    'ORDER_STATUS_CHANGED',
    'PAYMENT_UPDATED',
    'ACCESS_UPDATED',
    'RARE_BID_RECEIVED',
    'RARE_BID_SELECTED',
    'MANUFACTURER_REQUEST_CREATED',
    'MANUFACTURER_QUOTE_CREATED'
);

CREATE TYPE notification_device_platform AS ENUM (
    'ANDROID',
    'WEB'
);

CREATE TYPE notification_delivery_status AS ENUM (
    'PENDING',
    'SENT',
    'FAILED',
    'SKIPPED'
);

CREATE TABLE notification_preferences (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    in_app_enabled boolean NOT NULL DEFAULT TRUE,
    push_enabled boolean NOT NULL DEFAULT TRUE,
    order_created boolean NOT NULL DEFAULT TRUE,
    order_status_changed boolean NOT NULL DEFAULT TRUE,
    payment_updated boolean NOT NULL DEFAULT TRUE,
    access_updated boolean NOT NULL DEFAULT TRUE,
    rare_bid_received boolean NOT NULL DEFAULT TRUE,
    rare_bid_selected boolean NOT NULL DEFAULT TRUE,
    manufacturer_request_created boolean NOT NULL DEFAULT TRUE,
    manufacturer_quote_created boolean NOT NULL DEFAULT TRUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE notification_devices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform notification_device_platform NOT NULL,
    token text NOT NULL UNIQUE,
    device_label text,
    is_active boolean NOT NULL DEFAULT TRUE,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind notification_kind NOT NULL,
    title text NOT NULL,
    body text NOT NULL,
    payload_json jsonb NOT NULL,
    dedupe_key text UNIQUE,
    is_read boolean NOT NULL DEFAULT FALSE,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE notification_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id uuid NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    device_id uuid NOT NULL REFERENCES notification_devices(id) ON DELETE CASCADE,
    platform notification_device_platform NOT NULL,
    status notification_delivery_status NOT NULL DEFAULT 'PENDING',
    error_text text,
    created_at timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz
);

CREATE INDEX idx_notification_devices_user_active
    ON notification_devices (user_id, is_active);

CREATE INDEX idx_notifications_user_created
    ON notifications (user_id, created_at DESC, id DESC);

CREATE INDEX idx_notifications_user_unread_created
    ON notifications (user_id, is_read, created_at DESC, id DESC);

CREATE INDEX idx_notification_deliveries_notification_status
    ON notification_deliveries (notification_id, status);

CREATE TRIGGER trg_notification_preferences_updated_at
    BEFORE UPDATE ON notification_preferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_notification_devices_updated_at
    BEFORE UPDATE ON notification_devices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO notification_preferences (user_id)
SELECT id
FROM users
ON CONFLICT (user_id) DO NOTHING;

COMMIT;
