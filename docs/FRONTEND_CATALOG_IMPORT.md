# Frontend Catalog Import Contract

This document is for the frontend team implementing wholesaler Excel import and admin review.

## 1) What Changed
- `medicines` is no longer linked to platform `manufacturer` accounts.
- Medicine identity is based on:
  - `generic_name`
  - `brand_name`
  - `form`
  - `strength`
- `brand_name` is highly recommended.
- Backend now returns `warnings[]` when `brand_name` is empty.
- Admin can approve a new medicine candidate without sending `manufacturer_id`.

## 2) Wholesaler Flow
1. Parse Excel row in the app.
2. Call `POST /api/v1/medicines/validate`.
3. Read:
   - `status`
   - `warnings[]`
   - `matched_medicine`
   - `suggested_medicine`
   - `candidates[]`
   - `pending_candidate`
4. Decide UI action:
   - `MATCHED`
   - `SUGGESTED_MATCH`
   - `AMBIGUOUS`
   - `NEW_MEDICINE`
   - `PENDING_REVIEW`
5. If user confirms the row is a new medicine, call `POST /api/v1/medicine-candidates`.
6. Wait for admin review.
7. Only after admin approval can the app create an `offer`.

## 3) Validate API
- `POST /api/v1/medicines/validate`
- Auth: `Bearer <WHOLESALER_ACCESS_TOKEN>`

Request body:
```json
{
  "generic_name": "Paracetamol",
  "brand_name": "Panadol",
  "form": "Tablet",
  "strength": "500 mg",
  "pack_size": "20 tabs",
  "atc_code": "N02BE01"
}
```

Response shape:
```json
{
  "status": "MATCHED",
  "normalized": {
    "generic_name": "paracetamol",
    "brand_name": "panadol",
    "form": "tablet",
    "strength": "500 mg"
  },
  "warnings": [],
  "matched_medicine": {
    "id": "MEDICINE_UUID"
  },
  "suggested_medicine": null,
  "candidates": [],
  "pending_candidate": null
}
```

Status meanings:
- `MATCHED`
  - Exact normalized match exists.
  - Backend stops immediately at this stage.
  - Use `matched_medicine.id` for the next step.
- `SUGGESTED_MATCH`
  - No exact match, but backend found one strong likely match.
  - Show `suggested_medicine` first.
  - Still allow user to ignore and submit as new.
- `AMBIGUOUS`
  - Backend found multiple close candidates.
  - Show `candidates[]` and ask user to choose.
- `NEW_MEDICINE`
  - No exact match and no strong close candidate.
  - App can offer “submit as new medicine”.
- `PENDING_REVIEW`
  - Same medicine already exists in `medicine_candidates`.
  - Do not create another candidate.
  - Show waiting state.

Warnings:
- `BRAND_NAME_RECOMMENDED`
  - Request is still valid.
  - Frontend should visibly warn the user and encourage filling `brand_name`.
  - This warning is relevant only when exact catalog match did not already win.

## 4) Submit New Medicine Candidate
- `POST /api/v1/medicine-candidates`
- Auth: `Bearer <WHOLESALER_ACCESS_TOKEN>`

Request body:
```json
{
  "generic_name": "Ceftriaxone",
  "brand_name": "Ceftron",
  "form": "Injection",
  "strength": "1 g",
  "pack_size": "10 vials",
  "atc_code": "J01DD04",
  "force_submit": true
}
```

Rules:
- `force_submit = false`
  - backend blocks create when status is `SUGGESTED_MATCH` or `AMBIGUOUS`
- `force_submit = true`
  - user explicitly chooses to submit as a new medicine despite suggestions

Frontend rule:
- Only show the force-submit action after the user has seen existing suggestions.

## 5) Admin Review Flow

### List pending candidates
- `GET /api/v1/admin/medicine-candidates?status=PENDING&limit=50`
- Auth: `Bearer <ADMIN_ACCESS_TOKEN>`

### Approve by linking to an existing medicine
- `POST /api/v1/admin/medicine-candidates/:id/approve`

Request body:
```json
{
  "medicine_id": "EXISTING_MEDICINE_UUID",
  "decision_note": "Matched to existing catalog medicine"
}
```

### Approve by creating a new medicine
- `POST /api/v1/admin/medicine-candidates/:id/approve`

Request body:
```json
{
  "generic_name": "Амоксициллин",
  "brand_name": "Amoxil",
  "form": "Капсулы",
  "strength": "500 мг",
  "pack_size": "20 caps",
  "atc_code": "J01CA04",
  "is_active": true,
  "decision_note": "Approved as new medicine"
}
```

Notes:
- `medicine_id` present:
  - backend links candidate to existing medicine
- `medicine_id` absent:
  - backend creates a new `medicines` row from candidate data plus optional overrides
- no `manufacturer_id` is needed anymore

Approve response:
```json
{
  "candidate": {
    "id": "CANDIDATE_UUID",
    "status": "APPROVED",
    "matched_medicine_id": "MEDICINE_UUID",
    "reviewed_by": "ADMIN_UUID",
    "reviewed_at": "2026-03-16T12:00:00Z"
  },
  "medicine": {
    "id": "MEDICINE_UUID",
    "generic_name": "Амоксициллин",
    "brand_name": "Amoxil",
    "form": "Капсулы",
    "strength": "500 мг",
    "pack_size": "20 caps",
    "atc_code": "J01CA04",
    "is_active": true
  }
}
```

### Reject candidate
- `POST /api/v1/admin/medicine-candidates/:id/reject`

Request body:
```json
{
  "decision_note": "Use existing medicine instead"
}
```

## 6) Important Frontend Rules
- Do not create `offer` for a candidate that is still `PENDING`.
- Do not assume `brand_name` is mandatory, but always surface the backend warning.
- For `MATCHED`, use `matched_medicine.id`.
- For `PENDING_REVIEW`, show non-editable waiting state or navigation to review status.
- After admin approval, the app still needs a separate `offer` creation call.

## 7) Practical UI Mapping
- Wholesaler import screen:
  - parse rows
  - validate each row
  - show warnings
  - show candidate matches
  - allow submit-as-new with confirmation
- Admin review screen:
  - list pending candidates
  - open details
  - choose:
    - link to existing medicine
    - create new medicine
    - reject
