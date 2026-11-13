DROP TABLE IF EXISTS finance_entries;

ALTER TABLE products
  DROP COLUMN IF EXISTS selling_price,
  DROP COLUMN IF EXISTS cost_price;

