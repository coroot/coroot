-- Nord Outfitters demo shop schema + Coroot monitoring role
-- Runs against POSTGRES_DB (shop). Also install pg_stat_statements on `postgres`
-- because coroot-cluster-agent connects there by default.

CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'coroot') THEN
    CREATE ROLE coroot WITH LOGIN PASSWORD 'coroot';
  END IF;
END
$$;
GRANT pg_monitor TO coroot;

CREATE TABLE IF NOT EXISTS products (
  id    SERIAL PRIMARY KEY,
  name  TEXT NOT NULL,
  price INT  NOT NULL,
  stock INT  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS orders (
  order_id  TEXT PRIMARY KEY,
  total     INT  NOT NULL,
  currency  TEXT NOT NULL DEFAULT 'USD',
  email     TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tickets (
  id         SERIAL PRIMARY KEY,
  subject    TEXT NOT NULL,
  email      TEXT NOT NULL,
  status     TEXT NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tickets (id, subject, email, status) VALUES
  (1, 'Order delayed', 'alice@example.com', 'open'),
  (2, 'Wrong size shipped', 'bob@example.com', 'pending'),
  (3, 'Refund for Alpine Jacket', 'carol@example.com', 'resolved')
ON CONFLICT (id) DO UPDATE SET
  subject = EXCLUDED.subject,
  email = EXCLUDED.email,
  status = EXCLUDED.status;

SELECT setval(pg_get_serial_sequence('tickets', 'id'), (SELECT MAX(id) FROM tickets));

INSERT INTO products (id, name, price, stock) VALUES
  (1, 'Trail Runner', 129, 14),
  (2, 'City Pack', 89, 7),
  (3, 'Alpine Jacket', 210, 3),
  (4, 'Softshell Cap', 32, 40)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  price = EXCLUDED.price,
  stock = EXCLUDED.stock;

SELECT setval(pg_get_serial_sequence('products', 'id'), (SELECT MAX(id) FROM products));

GRANT CONNECT ON DATABASE shop TO coroot;
GRANT USAGE ON SCHEMA public TO coroot;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO coroot;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO coroot;

-- cluster-agent default DB
\connect postgres
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
GRANT CONNECT ON DATABASE postgres TO coroot;
