# NOTIFICATIONS

## Production-Ready Checklist
1. Standardize business events
   - Status: done
   - Implemented events:
     - `order.status_changed`
     - `payment.verified`
     - `access.updated`
     - `rare.bid_created`
     - `rare.bid_selected`
     - `manufacturer.request_created`
     - `manufacturer.quote_created`
   - `order.created` is currently inferred from `order.status_changed` with `old_status = null` and `new_status = CREATED`

2. Persistent in-app inbox
   - Status: done
   - Tables:
     - `notifications`
     - `notification_deliveries`

3. User-level notification preferences
   - Status: done
   - Table:
     - `notification_preferences`

4. Device/browser token registry
   - Status: done
   - Table:
     - `notification_devices`

5. Device management API
   - Status: done
   - Endpoints:
     - `GET /api/v1/notifications/devices`
     - `POST /api/v1/notifications/devices`
     - `DELETE /api/v1/notifications/devices/:id`

6. Read/unread sync API
   - Status: done
   - Endpoints:
     - `GET /api/v1/notifications`
     - `POST /api/v1/notifications/:id/read`
     - `POST /api/v1/notifications/read-all`

7. Push provider abstraction
   - Status: done
   - Providers:
     - `noop`
     - `fcm`

8. Android push delivery
   - Status: backend-ready
   - Requirement:
     - FCM token from Android app
     - FCM service account credentials on backend

9. Web push delivery
   - Status: backend-ready
   - Requirement:
     - FCM web token from browser app
     - FCM service account credentials on backend

10. Delivery logging and failure tracking
   - Status: done
   - Results stored in:
     - `notification_deliveries`

11. Invalid token handling
   - Status: done
   - If FCM returns an invalid/unregistered token error, backend marks the device inactive

12. Retry strategy
   - Status: partial
   - Current state:
     - failures are logged in DB
     - no scheduled retry worker for failed push deliveries yet

13. Provider credential management
   - Status: done
   - Environment:
     - `NOTIFICATION_PUSH_PROVIDER`
     - `FCM_CREDENTIALS_FILE`
     - `FCM_CREDENTIALS_JSON`
     - `FCM_DRY_RUN`

## Current Recommendation
For production, use:
- `NOTIFICATION_PUSH_PROVIDER=fcm`
- Android and Web clients should both register FCM tokens via `POST /api/v1/notifications/devices`

## Minimal Production Env
```env
NOTIFICATION_PUSH_PROVIDER=fcm
FCM_CREDENTIALS_FILE=/app/secrets/firebase-service-account.json
FCM_DRY_RUN=false
```

Or:
```env
NOTIFICATION_PUSH_PROVIDER=fcm
FCM_CREDENTIALS_JSON={"type":"service_account", ...}
FCM_DRY_RUN=false
```

## Frontend Contract
1. App/browser gets FCM token
2. Client sends token to backend:
```http
POST /api/v1/notifications/devices
Authorization: Bearer <ACCESS_TOKEN>
Content-Type: application/json
```
```json
{
  "platform": "ANDROID",
  "token": "<FCM_TOKEN>",
  "device_label": "Pixel 8"
}
```
3. Backend stores or refreshes token ownership
4. Worker receives business event from outbox
5. Backend writes inbox row
6. Backend attempts push delivery through FCM

## Notes
- In-app notifications and system push are separate channels.
- SSE is still useful for live open-screen updates.
- Push is the mechanism for closed-app/background notifications.
