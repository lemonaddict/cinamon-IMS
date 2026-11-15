-- Role: only admins can manage users. Existing rows become regular users.
ALTER TABLE users
  ADD COLUMN role text NOT NULL DEFAULT 'user'
    CHECK (role IN ('user', 'admin'));

CREATE INDEX users_role_idx ON users (role) WHERE role = 'admin';

-- Row Level Security as defense-in-depth.
-- Application already filters every query by auth.UserID(ctx) and uses composite FKs.
-- Policies are ENABLED but not FORCED so the table owner (typical free-tier DB role)
-- still works without a per-request transaction middleware. For stronger isolation
-- later: run the app as a non-owner role and ALTER TABLE ... FORCE ROW LEVEL SECURITY.
-- Policies use app.current_user_id when set by middleware; empty setting means "no access"
-- for non-owner roles.

CREATE OR REPLACE FUNCTION app_current_user_id() RETURNS bigint
  LANGUAGE sql STABLE AS $$
  SELECT nullif(current_setting('app.current_user_id', true), '')::bigint
$$;

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
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY tenant_isolation ON %I
         USING (user_id = app_current_user_id())
         WITH CHECK (user_id = app_current_user_id())', t);
  END LOOP;
END $$;
