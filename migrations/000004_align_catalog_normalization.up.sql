BEGIN;

CREATE OR REPLACE FUNCTION normalize_catalog_text(input text) RETURNS text AS $$
    SELECT NULLIF(
        trim(
            regexp_replace(
                lower(coalesce(input, '')),
                '[[:space:][:punct:]]+',
                ' ',
                'g'
            )
        ),
        ''
    );
$$ LANGUAGE sql IMMUTABLE;

COMMIT;
