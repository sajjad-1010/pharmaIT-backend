BEGIN;

DROP TRIGGER IF EXISTS trg_notification_devices_updated_at ON notification_devices;
DROP TRIGGER IF EXISTS trg_notification_preferences_updated_at ON notification_preferences;

DROP TABLE IF EXISTS notification_deliveries;
DROP TABLE IF EXISTS notification_devices;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS notification_preferences;

DROP TYPE IF EXISTS notification_delivery_status;
DROP TYPE IF EXISTS notification_device_platform;
DROP TYPE IF EXISTS notification_kind;

COMMIT;
