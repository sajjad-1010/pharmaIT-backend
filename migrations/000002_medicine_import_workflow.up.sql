BEGIN;

CREATE TYPE medicine_candidate_status AS ENUM ('PENDING', 'APPROVED', 'REJECTED');

CREATE FUNCTION normalize_catalog_text(input text) RETURNS text AS $$
    SELECT NULLIF(regexp_replace(lower(trim(coalesce(input, ''))), '\s+', ' ', 'g'), '');
$$ LANGUAGE sql IMMUTABLE;

CREATE UNIQUE INDEX uq_medicines_normalized_identity
    ON medicines (
        normalize_catalog_text(generic_name),
        COALESCE(normalize_catalog_text(brand_name), ''),
        normalize_catalog_text(form),
        COALESCE(normalize_catalog_text(strength), '')
    );

CREATE TABLE medicine_candidates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    wholesaler_id uuid NOT NULL REFERENCES wholesalers(user_id) ON DELETE CASCADE,
    generic_name text NOT NULL,
    brand_name text,
    form text NOT NULL,
    strength text,
    pack_size text,
    atc_code text,
    normalized_generic_name text NOT NULL,
    normalized_brand_name text,
    normalized_form text NOT NULL,
    normalized_strength text,
    status medicine_candidate_status NOT NULL DEFAULT 'PENDING',
    matched_medicine_id uuid REFERENCES medicines(id) ON DELETE SET NULL,
    admin_decision_note text,
    reviewed_by uuid REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_medicine_candidates_status_created
    ON medicine_candidates (status, created_at DESC);

CREATE INDEX idx_medicine_candidates_wh_status_created
    ON medicine_candidates (wholesaler_id, status, created_at DESC);

CREATE UNIQUE INDEX uq_medicine_candidates_pending_identity
    ON medicine_candidates (
        normalized_generic_name,
        COALESCE(normalized_brand_name, ''),
        normalized_form,
        COALESCE(normalized_strength, '')
    )
    WHERE status = 'PENDING';

CREATE TRIGGER trg_medicine_candidates_updated_at
    BEFORE UPDATE ON medicine_candidates
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

COMMIT;
