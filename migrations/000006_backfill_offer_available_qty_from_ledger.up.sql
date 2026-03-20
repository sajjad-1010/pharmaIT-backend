BEGIN;

WITH ledger AS (
    SELECT
        wholesaler_id,
        medicine_id,
        GREATEST(
            COALESCE(SUM(
                CASE type
                    WHEN 'IN' THEN qty
                    WHEN 'RELEASED' THEN qty
                    WHEN 'ADJUST' THEN qty
                    WHEN 'OUT' THEN -qty
                    WHEN 'RESERVED' THEN -qty
                    ELSE 0
                END
            ), 0),
            0
        ) AS available_qty
    FROM inventory_movements
    GROUP BY wholesaler_id, medicine_id
)
UPDATE wholesaler_offers o
SET available_qty = COALESCE(l.available_qty, 0)
FROM (
    SELECT
        o2.id,
        l2.available_qty
    FROM wholesaler_offers o2
    LEFT JOIN ledger l2
        ON l2.wholesaler_id = o2.wholesaler_id
       AND l2.medicine_id = o2.medicine_id
) AS l
WHERE o.id = l.id;

COMMIT;
