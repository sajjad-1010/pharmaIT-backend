-- Destructive migration.
-- Down migration intentionally left as no-op because restoring the previous
-- medicine-centric schema would require reconstructing dropped catalog tables
-- and data that no longer exist.
SELECT 1;
