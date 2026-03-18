BEGIN;

CREATE OR REPLACE FUNCTION normalize_catalog_text(input text) RETURNS text AS $$
    SELECT NULLIF(regexp_replace(lower(trim(coalesce(input, ''))), '\s+', ' ', 'g'), '');
$$ LANGUAGE sql IMMUTABLE;

COMMIT;
