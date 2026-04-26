# Frontend API Contract

This document is the current contract for the frontend team. It reflects the active runtime backend after the offer-centric refactor.

## 1) Core Model

The backend is no longer medicine-catalog based.

The primary entity shown to users is:
- `wholesaler_offers`

Meaning:
- every imported Excel row becomes one offer row
- backend generates its own `id`
- backend does not run medicine matching, dedupe, or admin review

## 2) Offer Fields

### Required
- `name`
- `display_price`

### Optional
- `producer`
- `expiry_date`
- `is_active`

### Not part of public offer input anymore
- `currency`
- `min_order_qty`
- `delivery_eta_hours`

Notes:
- search is performed only on `name`

## 3) Offer API

### List offers
- `GET /api/v1/offers?query=&limit=&cursor=`
- public

Query params:
- `query`: optional text search on `name`
- `limit`: optional, default `20`, max `100`
- `cursor`: optional, `base64(timestamp|id)`

Response:
```json
{
  "items": [
    {
      "ID": "OFFER_UUID",
      "WholesalerID": "WHOLESALER_UUID",
      "Name": "L-тироксин 50 Б/Хеми тб 50мкг №50",
      "Producer": "Берлин Хеми",
      "DisplayPrice": "12.8000",
      "ExpiryDate": "2028-02-01T00:00:00Z",
      "IsActive": true,
      "CreatedAt": "2026-03-23T10:00:00Z",
      "UpdatedAt": "2026-03-23T10:05:00Z"
    }
  ],
  "next_cursor": null,
  "has_more": false
}
```

Frontend rules:
- use `query` for search
- show the exact backend `name`
- do not try to split `name` into dose/form/count unless UI really needs parsing

### List current wholesaler offers
- `GET /api/v1/offers/mine?query=&limit=&cursor=`
- auth required
- role: `WHOLESALER`

Use this endpoint in wholesaler dashboards/panels so the app does not fetch every offer in the system.

Query params:
- `query`: optional text search on `name`
- `limit`: optional, default `20`, max `100`
- `cursor`: optional, `base64(timestamp|id)`

Response shape is the same as public `/offers`.

### Create offer
- `POST /api/v1/offers`
- auth required
- role: `WHOLESALER`

Request:
```json
{
  "name": "03 Гель бальзам для тела \"Глюк,Хонд,Саб\" 75мл туба",
  "display_price": "11.5000",
  "producer": "Мирролла",
  "expiry_date": "2028-09-01",
  "is_active": true
}
```

### Update offer
- `PATCH /api/v1/offers/:id`
- auth required
- role: `WHOLESALER`

Patch body:
```json
{
  "display_price": "13.0000",
  "producer": "Мирролла",
  "expiry_date": "2028-10-01",
  "is_active": true
}
```

Important:
- frontend must not send stock-related fields in offer create/update

### Batch create offers
- `POST /api/v1/offers/batch`
- auth required
- role: `WHOLESALER`

Use this endpoint for Excel import or large bulk loads.

Request:
```json
{
  "items": [
    {
      "name": "03 Гель бальзам для тела \"Глюк,Хонд,Саб\" 75мл туба",
      "display_price": "11.5000",
      "producer": "Мирролла",
      "expiry_date": "2028-09-01",
      "is_active": true
    },
    {
      "name": "L-тироксин 50 Б/Хеми тб 50мкг №50",
      "display_price": "12.8000",
      "producer": "Берлин Хеми"
    }
  ]
}
```

Response:
```json
{
  "created_count": 2,
  "failed_count": 0,
  "items": [
    {
      "ID": "OFFER_UUID_1"
    },
    {
      "ID": "OFFER_UUID_2"
    }
  ],
  "errors": []
}
```

Partial failure shape:
```json
{
  "created_count": 1,
  "failed_count": 1,
  "items": [
    {
      "ID": "OFFER_UUID_1"
    }
  ],
  "errors": [
    {
      "index": 1,
      "code": "INVALID_DISPLAY_PRICE",
      "message": "display_price must be non-negative decimal"
    }
  ]
}
```

Batch rules:
- one request can contain many rows
- backend validates each row independently
- invalid rows are returned in `errors[]` with zero-based `index`
- valid rows are still inserted
- current hard limit per request: `10000` rows

## 4) Search Behavior

Search works directly on offer `name`.

This means these kinds of rows are valid and searchable as raw text:
- `03 Гель бальзам для тела "Глюк,Хонд,Саб" 75мл туба`
- `L-тироксин 50 Б/Хеми тб 50мкг №50`

Frontend guidance:
- keep the search box simple
- send the exact user query to `query`
- backend handles substring/fuzzy matching

## 5) Stock
Stock was removed from the active backend flow.

Frontend rules:
- there are no active inventory endpoints in runtime
- do not show stock as a backend-guaranteed field
- order creation no longer performs backend stock validation

## 6) Orders

### Create order
- `POST /api/v1/orders`
- auth required
- role: `PHARMACY`

Request:
```json
{
  "wholesaler_id": "WHOLESALER_UUID",
  "currency": "TJS",
  "items": [
    {
      "offer_id": "OFFER_UUID",
      "qty": 2
    }
  ]
}
```

Notes:
- orders are created from `offer_id`
- backend snapshots order item data at order time

### Order list response
- `GET /api/v1/orders?limit=&cursor=`
- auth required
- roles:
  - `PHARMACY`
  - `WHOLESALER`
  - `ADMIN`

Order object includes:
- `ID`
- `PharmacyID`
- `PharmacyName`
- `PharmacyCity`
- `PharmacyAddress`
- `PharmacyLicenseNo`
- `PharmacyEmail`
- `PharmacyPhone`
- `WholesalerID`
- `Status`
- `TotalAmount`
- `Currency`
- `Items[]`
- `CreatedAt`
- `UpdatedAt`

Order item object includes:
- `ID`
- `OfferID`
- `ItemName`
- `Producer`
- `Qty`
- `UnitPrice`
- `LineTotal`

Frontend rule:
- for wholesaler order list, use `PharmacyName` and contact fields directly
- use `Items[]` directly in list/detail UI
- no extra lookup is needed for order items

### Update order status
- `PATCH /api/v1/orders/:id/status`
- auth required
- role depends on backend policy

Request:
```json
{
  "status": "CONFIRMED"
}
```

Allowed statuses:
- `CREATED`
- `CONFIRMED`
- `PACKING`
- `SHIPPED`
- `DELIVERED`
- `CANCELED`


## 6.1) Rare Requests

Rare request routes are active in runtime.

### Create rare request
- `POST /api/v1/rare-requests`
- auth required
- role: `PHARMACY`

Request:
```json
{
  "requested_name_text": "????? ?????? ????????",
  "qty": 3,
  "deadline_at": "2026-03-27T12:00:00Z",
  "notes": "urgent"
}
```

### List rare requests
- `GET /api/v1/rare-requests?status=&limit=&cursor=`
- auth required

### Create rare bid
- `POST /api/v1/rare-requests/:id/bids`
- auth required
- role: `WHOLESALER`

Request:
```json
{
  "price": "12.0000",
  "currency": "TJS",
  "available_qty": 20,
  "delivery_eta_hours": 18,
  "notes": "available"
}
```

### Select rare bid
- `POST /api/v1/rare-requests/bids/:id/select`
- auth required
- role: `PHARMACY`
## 7) Notifications

### In-app notifications
- `GET /api/v1/notifications`
- `POST /api/v1/notifications/:id/read`
- `POST /api/v1/notifications/read-all`
- `GET /api/v1/notifications/preferences`
- `PUT /api/v1/notifications/preferences`

### Device registration
- `POST /api/v1/notifications/devices`
- `GET /api/v1/notifications/devices`
- `DELETE /api/v1/notifications/devices/:id`

Register device example:
```json
{
  "platform": "ANDROID",
  "token": "FCM_TOKEN",
  "device_label": "Pixel 8"
}
```

Current product decision:
- Android app: push notifications can be used
- Web: use only in-app notifications for now

## 8) SSE / Realtime

### Stream endpoint
- `GET /api/v1/stream/offers`

Current realtime event types:
- `offer.updated`
- `order.status_changed`

Use SSE for:
- live order-status refresh
- live offer changes while the app is open

Do not rely on SSE for closed-app notifications.

## 9) Standard Error Format

All backend errors:
```json
{
  "error": {
    "code": "STRING",
    "message": "STRING",
    "details": {}
  }
}
```

Frontend rule:
- show `message`

## 10) Frontend Checklist

### Pharmacy
- search offers by `query`
- read exact `name` from backend
- create order using `offer_id`
- show order item snapshots from `Items[]`
- show in-app notifications

### Wholesaler
- create/update offers with `name` and `display_price`
- read pharmacy identity/contact fields from order responses
- use notifications + SSE

### Web
- use in-app notifications only
- no system push required

### Android
- register FCM token through `/notifications/devices`
- use in-app feed plus push

