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

## 2) Catalog + Search
```bash
curl "http://localhost/api/v1/medicines?query=para&limit=20&cursor="
```

## 3) Offers

### Create Offer (Wholesaler)
```bash
curl -X POST http://localhost/api/v1/offers \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "medicine_id":"<MEDICINE_ID>",
    "display_price":"10.5000",
    "currency":"TJS",
    "available_qty":120,
    "min_order_qty":5,
    "delivery_eta_hours":24,
    "is_active":true
  }'
```

### List Offers (Cursor)
```bash
curl "http://localhost/api/v1/offers?medicine_id=<MEDICINE_ID>&limit=20&cursor=<CURSOR>"
```

## 4) Inventory

### Add Movement
```bash
curl -X POST http://localhost/api/v1/inventory/movements \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "medicine_id":"<MEDICINE_ID>",
    "type":"IN",
    "qty":500,
    "ref_type":"manual_adjust"
  }'
```

### Get Stock
```bash
curl "http://localhost/api/v1/inventory/stock?medicine_id=<MEDICINE_ID>" \
  -H "Authorization: Bearer <WHOLESALER_ACCESS_TOKEN>"
```

## 5) Orders

### Create Order (Pharmacy)
```bash
curl -X POST http://localhost/api/v1/orders \
  -H "Authorization: Bearer <PHARMACY_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "wholesaler_id":"<WHOLESALER_ID>",
    "currency":"TJS",
    "items":[{"offer_id":"<OFFER_ID>","qty":10}]
  }'
```

### List Orders
```bash
curl "http://localhost/api/v1/orders?limit=20&cursor=" \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

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
    "currency":"TJS",
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
    "currency":"TJS",
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

## 10) Flutter Testing Checklist
1. Login and store access/refresh tokens securely.
2. Call `/auth/me` and route UI by role.
3. Test cursor pagination on `/medicines`, `/offers`, `/orders`.
4. Open SSE stream and verify live updates on offer/stock changes.
5. Create order and check stock reduction.
6. Run payment flow and confirm access extension.
7. Verify standard backend error format handling in UI.
