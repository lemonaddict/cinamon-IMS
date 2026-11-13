ALTER TABLE products
  ADD COLUMN cost_price bigint NOT NULL DEFAULT 0 CHECK (cost_price >= 0),
  ADD COLUMN selling_price bigint NOT NULL DEFAULT 0 CHECK (selling_price >= 0);

CREATE TABLE finance_entries (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id),
  entry_type text NOT NULL CHECK (entry_type IN ('income', 'expense')),
  category text NOT NULL,
  amount bigint NOT NULL CHECK (amount > 0),
  entry_date date NOT NULL DEFAULT CURRENT_DATE,
  notes text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX finance_entries_user_date_idx ON finance_entries (user_id, entry_date DESC, id DESC);

