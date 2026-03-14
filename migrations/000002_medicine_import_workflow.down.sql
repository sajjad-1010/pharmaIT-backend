BEGIN;

DROP TRIGGER IF EXISTS trg_medicine_candidates_updated_at ON medicine_candidates;
DROP INDEX IF EXISTS uq_medicine_candidates_pending_identity;
DROP INDEX IF EXISTS idx_medicine_candidates_wh_status_created;
DROP INDEX IF EXISTS idx_medicine_candidates_status_created;
DROP TABLE IF EXISTS medicine_candidates;

DROP INDEX IF EXISTS uq_medicines_normalized_identity;
DROP FUNCTION IF EXISTS normalize_catalog_text(text);
DROP TYPE IF EXISTS medicine_candidate_status;

COMMIT;
