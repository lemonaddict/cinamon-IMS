CREATE TABLE users (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name text NOT NULL,
  email text NOT NULL,
  password_hash text NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_uq ON users (lower(email));

CREATE TABLE sessions (
  token_hash bytea PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE categories (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id),
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, id),
  UNIQUE (user_id, name)
);

CREATE TABLE locations (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id),
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, id),
  UNIQUE (user_id, name)
);

CREATE TABLE products (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id),
  category_id bigint,
  name text NOT NULL,
  sku text NOT NULL,
  description text NOT NULL DEFAULT '',
  image_path text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, id),
  UNIQUE (user_id, sku),
  FOREIGN KEY (user_id, category_id) REFERENCES categories (user_id, id)
);
CREATE INDEX products_category_idx ON products (user_id, category_id);

-- Snapshot of current stock; the ledger below is the source of history.
CREATE TABLE inventory_balances (
  user_id bigint NOT NULL,
  product_id bigint NOT NULL,
  location_id bigint NOT NULL,
  quantity bigint NOT NULL DEFAULT 0 CHECK (quantity >= 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, product_id, location_id),
  FOREIGN KEY (user_id, product_id) REFERENCES products (user_id, id),
  FOREIGN KEY (user_id, location_id) REFERENCES locations (user_id, id)
);

CREATE TABLE stock_movements (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id),
  movement_type text NOT NULL CHECK (movement_type IN ('in','out','adjustment')),
  reference_number text,
  notes text,
  idempotency_key text,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, id),
  UNIQUE (user_id, idempotency_key)
);
CREATE INDEX stock_movements_user_idx ON stock_movements (user_id, id DESC);

CREATE TABLE stock_movement_items (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id bigint NOT NULL,
  stock_movement_id bigint NOT NULL,
  product_id bigint NOT NULL,
  location_id bigint NOT NULL,
  quantity bigint NOT NULL CHECK (quantity > 0),
  direction text NOT NULL CHECK (direction IN ('in','out')),
  FOREIGN KEY (user_id, stock_movement_id) REFERENCES stock_movements (user_id, id),
  FOREIGN KEY (user_id, product_id) REFERENCES products (user_id, id),
  FOREIGN KEY (user_id, location_id) REFERENCES locations (user_id, id)
);
CREATE INDEX smi_movement_idx ON stock_movement_items (user_id, stock_movement_id);
CREATE INDEX smi_product_idx ON stock_movement_items (user_id, product_id);

CREATE FUNCTION forbid_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION '% is append-only', TG_TABLE_NAME; END $$;
CREATE TRIGGER stock_movements_immutable BEFORE UPDATE OR DELETE ON stock_movements
  FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TRIGGER stock_movement_items_immutable BEFORE UPDATE OR DELETE ON stock_movement_items
  FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
