-- +goose Up

CREATE TABLE users (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  telegram_id           bigint NOT NULL UNIQUE,
  first_name            text NOT NULL DEFAULT '',
  last_name             text NOT NULL DEFAULT '',
  username              text NOT NULL DEFAULT '',
  phone                 text,
  role                  text NOT NULL DEFAULT 'customer' CHECK (role IN ('customer', 'admin', 'owner')),
  is_active             boolean NOT NULL DEFAULT true,
  notifications_enabled boolean NOT NULL DEFAULT true,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  last_login_at         timestamptz
);

-- Telegram deep-link login: the browser/app keeps `secret`, the bot only ever sees `code`.
CREATE TABLE login_requests (
  code        text PRIMARY KEY,
  secret_hash text NOT NULL,
  status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'consumed')),
  user_id     uuid REFERENCES users (id) ON DELETE CASCADE,
  expires_at  timestamptz NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

CREATE TABLE admin_invites (
  code       text PRIMARY KEY,
  created_by uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  used_by    uuid REFERENCES users (id) ON DELETE SET NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE files (
  id         uuid PRIMARY KEY,
  kind       text NOT NULL CHECK (kind IN ('product_image', 'category_image', 'receipt')),
  owner_id   uuid REFERENCES users (id) ON DELETE SET NULL,
  variants   jsonb NOT NULL DEFAULT '{}'::jsonb, -- name -> storage key (thumb/card/full)
  mime       text NOT NULL,
  size_bytes bigint NOT NULL,
  width      int NOT NULL,
  height     int NOT NULL,
  sha256     text NOT NULL,
  dhash      bigint,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX files_receipt_sha_idx ON files (sha256) WHERE kind = 'receipt';

CREATE TABLE categories (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name          text NOT NULL,
  slug          text NOT NULL UNIQUE,
  image_file_id uuid REFERENCES files (id) ON DELETE SET NULL,
  position      int NOT NULL DEFAULT 0,
  is_active     boolean NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE products (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  category_id  uuid NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
  name         text NOT NULL,
  slug         text NOT NULL UNIQUE,
  brand        text NOT NULL DEFAULT '',
  description  text NOT NULL DEFAULT '', -- markdown, raw HTML stripped on save
  ingredients  text NOT NULL DEFAULT '',
  how_to_use   text NOT NULL DEFAULT '',
  option_label text NOT NULL DEFAULT '', -- e.g. "Tus", "Hajm"; empty for single-variant products
  status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
  is_new       boolean NOT NULL DEFAULT false,
  is_featured  boolean NOT NULL DEFAULT false,
  created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX products_listing_idx ON products (status, category_id, created_at DESC);

CREATE TABLE product_variants (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id    uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
  label         text NOT NULL DEFAULT '',
  sku           text UNIQUE,
  price         bigint NOT NULL CHECK (price > 0),           -- so'm
  old_price     bigint CHECK (old_price IS NULL OR old_price > 0),
  stock         int NOT NULL DEFAULT 0 CHECK (stock >= 0),
  image_file_id uuid REFERENCES files (id) ON DELETE SET NULL,
  position      int NOT NULL DEFAULT 0,
  is_active     boolean NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX product_variants_product_idx ON product_variants (product_id);

CREATE TABLE product_images (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
  file_id    uuid NOT NULL REFERENCES files (id) ON DELETE RESTRICT,
  position   int NOT NULL DEFAULT 0,
  UNIQUE (product_id, file_id)
);

CREATE TABLE orders (
  id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  number                  bigint GENERATED ALWAYS AS IDENTITY (START WITH 1001) UNIQUE,
  user_id                 uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  status                  text NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'confirmed', 'shipping', 'delivered', 'rejected')),
  customer_name           text NOT NULL,
  phone                   text NOT NULL,
  address_text            text NOT NULL,
  lat                     double precision NOT NULL,
  lng                     double precision NOT NULL,
  customer_comment        text NOT NULL DEFAULT '',
  total_amount            bigint NOT NULL CHECK (total_amount >= 0),
  receipt_file_id         uuid NOT NULL UNIQUE REFERENCES files (id) ON DELETE RESTRICT,
  receipt_duplicate_of    uuid REFERENCES orders (id) ON DELETE SET NULL,
  receipt_duplicate_kind  text CHECK (receipt_duplicate_kind IN ('exact', 'similar')),
  assigned_admin_id       uuid REFERENCES users (id) ON DELETE SET NULL,
  reject_reason           text NOT NULL DEFAULT '',
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX orders_user_idx ON orders (user_id, created_at DESC);
CREATE INDEX orders_status_idx ON orders (status, created_at DESC);

CREATE TABLE order_items (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id      uuid NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
  product_id    uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
  variant_id    uuid NOT NULL REFERENCES product_variants (id) ON DELETE RESTRICT,
  product_name  text NOT NULL,   -- snapshot
  variant_label text NOT NULL DEFAULT '',
  sku           text NOT NULL DEFAULT '',
  image_file_id uuid REFERENCES files (id) ON DELETE SET NULL,
  unit_price    bigint NOT NULL,
  quantity      int NOT NULL CHECK (quantity > 0)
);
CREATE INDEX order_items_order_idx ON order_items (order_id);

CREATE TABLE order_status_history (
  id          bigserial PRIMARY KEY,
  order_id    uuid NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
  from_status text,
  to_status   text NOT NULL,
  changed_by  uuid REFERENCES users (id) ON DELETE SET NULL,
  comment     text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_history_order_idx ON order_status_history (order_id, id);

CREATE TABLE order_notes (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id   uuid NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
  author_id  uuid REFERENCES users (id) ON DELETE SET NULL,
  body       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_notes_order_idx ON order_notes (order_id, created_at);

-- Telegram messages sent to admins about an order, so they can be edited when it is claimed.
CREATE TABLE order_admin_messages (
  id         bigserial PRIMARY KEY,
  order_id   uuid NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
  chat_id    bigint NOT NULL,
  message_id bigint NOT NULL,
  UNIQUE (order_id, chat_id)
);

CREATE TABLE settings (
  key        text PRIMARY KEY,
  value      text NOT NULL,
  updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_log (
  id         bigserial PRIMARY KEY,
  actor_id   uuid REFERENCES users (id) ON DELETE SET NULL,
  action     text NOT NULL,
  entity     text NOT NULL,
  entity_id  text NOT NULL DEFAULT '',
  meta       jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_log, settings, order_admin_messages, order_notes, order_status_history,
  order_items, orders, product_images, product_variants, products, categories, files,
  admin_invites, refresh_tokens, login_requests, users CASCADE;
