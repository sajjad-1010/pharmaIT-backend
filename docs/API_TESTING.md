# API TESTING

Base URL (via nginx):
- `http://localhost`
- API prefix: `/api/v1`

## 1) Auth Flow

### Register Pharmacy
```bash
curl -X POST http://localhost/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email":"pharmacy@example.com",
    "password":"Passw0rd!",
    "role":"PHARMACY",
    "profile":{"name":"City Pharmacy","city":"Dushanbe"}
  }'
```

### Admin Activate User
```bash
curl -X PATCH http://localhost/api/v1/admin/users/<USER_ID>/status \
  -H "Authorization: Bearer <ADMIN_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"status":"ACTIVE"}'
```

### Login
```bash
curl -X POST http://localhost/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"identifier":"pharmacy@example.com","password":"Passw0rd!"}'
```

### Refresh
```bash
curl -X POST http://localhost/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<REFRESH_TOKEN>"}'
```

### Me
```bash
curl http://localhost/api/v1/auth/me \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

## 2) Offers + Search

### Create Offer (Wholesaler)
```bash
curl -X POST http://localhost/api/v1/offers \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "name":"L-тироксин 50 Б/Хеми тб 50мкг №50",
    "display_price":"10.5000",
    "producer":"Берлин Хеми",
    "expiry_date":"2027-09-15",
    "is_active":true
  }'
```

Offer contract note:
- `available_qty` is not accepted by the public offer API anymore.
- Stock changes must go through `POST /api/v1/inventory/movements`.
- `delivery_eta_hours` was removed from regular offers. Keep ETA only in rare bid submissions.
- Search is performed by `name`.

### Batch Create Offers (Wholesaler)
```bash
curl -X POST http://localhost/api/v1/offers/batch \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "items":[
      {
        "name":"03 Гель бальзам для тела \"Глюк,Хонд,Саб\" 75мл туба",
        "display_price":"11.5000",
        "producer":"Мирролла",
        "expiry_date":"2028-09-01",
        "is_active":true
      },
      {
        "name":"L-тироксин 50 Б/Хеми тб 50мкг №50",
        "display_price":"12.8000",
        "producer":"Берлин Хеми"
      }
    ]
  }'
```

Batch response note:
- valid rows are inserted
- invalid rows are returned in `errors[]`
- `errors[].index` is zero-based and maps to the original row position in the batch payload
- max batch size is `10000`

### List Offers (Cursor)
```bash
curl "http://localhost/api/v1/offers?query=тироксин&limit=20&cursor=<CURSOR>"
```

## 3) Inventory

### Add Movement
```bash
curl -X POST http://localhost/api/v1/inventory/movements \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "offer_id":"<OFFER_ID>",
    "type":"IN",
    "qty":500,
    "ref_type":"manual_adjust"
  }'
```

### Get Stock
```bash
curl "http://localhost/api/v1/inventory/stock?offer_id=<OFFER_ID>" \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>"
```

## 4) Orders

### Create Order (Pharmacy)
```bash
curl -X POST http://localhost/api/v1/orders \
  -H "Authorization: Bearer <PHARMACY_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "wholesaler_id":"<WHOLESALER_ID>",
    "items":[{"offer_id":"<OFFER_ID>","qty":10}]
  }'
```

### List Orders
```bash
curl "http://localhost/api/v1/orders?limit=20&cursor=" \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

Order response note:
- order objects now include:
  - `PharmacyName`
  - `PharmacyCity`
  - `PharmacyAddress`
  - `PharmacyLicenseNo`
  - `PharmacyEmail`
  - `PharmacyPhone`
- order objects in list responses now also include `Items[]` with:
  - `OfferID`
  - `ItemName`
  - `Producer`
  - `Qty`
  - `UnitPrice`
  - `LineTotal`
- wholesaler UIs can use these fields directly instead of deriving a fallback from `PharmacyID`.

### Update Order Status
```bash
curl -X PATCH http://localhost/api/v1/orders/<ORDER_ID>/status \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"status":"CONFIRMED"}'
```

## 6) Rare Request Flow

### Create Rare Request (Pharmacy)
```bash
curl -X POST http://localhost/api/v1/rare-requests \
  -H "Authorization: Bearer <PHARMACY_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "requested_name_text":"Rare Medicine X",
    "qty":20,
    "deadline_at":"2026-03-03T10:00:00Z"
  }'
```

### Submit Bid (Wholesaler)
```bash
curl -X POST http://localhost/api/v1/rare-requests/<RARE_REQUEST_ID>/bids \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "price":"12.0000",
    "available_qty":20,
    "delivery_eta_hours":18
  }'
```

### Select Bid (Pharmacy)
```bash
curl -X POST http://localhost/api/v1/rare-requests/bids/<BID_ID>/select \
  -H "Authorization: Bearer <PHARMACY_ACCESS_TOKEN>"
```

## 7) Manufacturer Flow

### Create Manufacturer Request (Wholesaler)
```bash
curl -X POST http://localhost/api/v1/manufacturer-requests \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "manufacturer_id":"<MANUFACTURER_ID>",
    "requested_name_text":"Amoxicillin bulk",
    "qty":10000
  }'
```

### Submit Quote (Manufacturer)
```bash
curl -X POST http://localhost/api/v1/manufacturer-requests/<REQUEST_ID>/quotes \
  -H "Authorization: Bearer <MANUFACTURER_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "unit_price_final":"7.9000",
    "lead_time_days":12
  }'
```

## 8) Payments + Access

### Create Invoice
```bash
curl -X POST http://localhost/api/v1/payments/invoice \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

### Webhook (Mock)
```bash
curl -X POST http://localhost/api/v1/payments/webhook \
  -H "X-Signature: <HMAC_HEX>" \
  -H "Content-Type: application/json" \
  -d '{
    "invoice_id":"<INVOICE_ID>",
    "transaction_id":"TX123",
    "status":"PAID"
  }'
```

### Get Access
```bash
curl http://localhost/api/v1/payments/access \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

## 9) SSE
```bash
curl -N http://localhost/api/v1/stream/offers
```
Expected events:
- `offer.updated`
- `inventory.changed`
- `order.status_changed`

## 10) Notifications

### Register Device Token
```bash
curl -X POST http://localhost/api/v1/notifications/devices \
  -H "Authorization: Bearer <ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "platform":"ANDROID",
    "token":"demo-device-token-001",
    "device_label":"Pixel 8"
  }'
```

### List Registered Devices
```bash
curl http://localhost/api/v1/notifications/devices \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

### Get Notification Preferences
```bash
curl http://localhost/api/v1/notifications/preferences \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

### Update Notification Preferences
```bash
curl -X PUT http://localhost/api/v1/notifications/preferences \
  -H "Authorization: Bearer <ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "push_enabled": true,
    "order_created": true,
    "order_status_changed": true
  }'
```

### List Notifications
```bash
curl "http://localhost/api/v1/notifications?limit=20&cursor=&unread_only=false" \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

### Mark Notification Read
```bash
curl -X POST http://localhost/api/v1/notifications/<NOTIFICATION_ID>/read \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

### Mark All Read
```bash
curl -X POST http://localhost/api/v1/notifications/read-all \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

Notification foundation note:
- backend now stores notification inbox rows in PostgreSQL
- device tokens/subscriptions are stored in `notification_devices`
- push delivery logging is stored in `notification_deliveries`
- backend supports `noop` and `fcm` push providers
- for real Android/Web push, set:
  - `NOTIFICATION_PUSH_PROVIDER=fcm`
  - `FCM_CREDENTIALS_FILE=/path/to/firebase-service-account.json`
  - or `FCM_CREDENTIALS_JSON=<raw json>`
- invalid FCM tokens are automatically deactivated after provider failure detection

## 11) Flutter Testing Checklist
1. Login and store access/refresh tokens securely.
2. Call `/auth/me` and route UI by role.
3. Test cursor pagination on `/medicines`, `/offers`, `/orders`.
4. Open SSE stream and verify live updates on offer/stock/order-status changes.
5. Create order and check stock reduction.
6. Register device token and verify notifications list updates after events.
7. Run payment flow and confirm access extension.
8. Verify standard backend error format handling in UI.

