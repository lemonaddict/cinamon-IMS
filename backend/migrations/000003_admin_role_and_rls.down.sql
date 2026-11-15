DO $$
DECLARE
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'categories', 'locations', 'products',
    'inventory_balances', 'stock_movements', 'stock_movement_items',
    'finance_entries'
  ]
  LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END $$;

DROP FUNCTION IF EXISTS app_current_user_id();

ALTER TABLE users DROP COLUMN IF EXISTS role;
