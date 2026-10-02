CREATE TABLE users (
  id           BIGSERIAL PRIMARY KEY,
  issuer       TEXT NOT NULL,
  subject      TEXT NOT NULL,
  display_name TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (issuer, subject)
);

CREATE TABLE sessions (
  token_hash BYTEA PRIMARY KEY,                 -- SHA-256 ของ token
  user_id    BIGINT NOT NULL REFERENCES users(id),
  id_token   TEXT NOT NULL,                     -- ใช้ตอน logout เท่านั้น
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE restaurants (
  id                    BIGSERIAL PRIMARY KEY,
  owner_id              BIGINT NOT NULL REFERENCES users(id),
  name                  TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 80),
  description           TEXT NOT NULL CHECK (char_length(btrim(description)) BETWEEN 1 AND 2000),
  capacity              INT  NOT NULL CHECK (capacity BETWEEN 1 AND 500),
  open_time             TIME NOT NULL,
  close_time            TIME NOT NULL,
  cancel_window_minutes INT  NOT NULL DEFAULT 30 CHECK (cancel_window_minutes >= 30),
  min_lead_minutes      INT  NOT NULL DEFAULT 120,
  accepting_bookings    BOOLEAN NOT NULL DEFAULT TRUE,
  deleted_at            TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (open_time <> close_time),
  CHECK (EXTRACT(MINUTE FROM open_time)  IN (0, 30) AND EXTRACT(SECOND FROM open_time)  = 0),
  CHECK (EXTRACT(MINUTE FROM close_time) IN (0, 30) AND EXTRACT(SECOND FROM close_time) = 0),
  CHECK (min_lead_minutes >= cancel_window_minutes)
);
CREATE UNIQUE INDEX restaurants_owner_name_uq
  ON restaurants (owner_id, lower(btrim(name))) WHERE deleted_at IS NULL;
CREATE INDEX restaurants_owner_idx ON restaurants (owner_id);

CREATE TABLE categories (
  id         SMALLINT PRIMARY KEY,
  slug       TEXT NOT NULL UNIQUE,
  name_th    TEXT NOT NULL,
  sort_order SMALLINT NOT NULL
);

CREATE TABLE restaurant_categories (
  restaurant_id BIGINT   NOT NULL REFERENCES restaurants(id),
  category_id   SMALLINT NOT NULL REFERENCES categories(id),
  PRIMARY KEY (restaurant_id, category_id)
);

CREATE TABLE restaurant_images (
  id            BIGSERIAL PRIMARY KEY,
  restaurant_id BIGINT NOT NULL REFERENCES restaurants(id),
  content_type  TEXT   NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
  data          BYTEA  NOT NULL CHECK (octet_length(data) BETWEEN 1 AND 2097152),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX restaurant_images_restaurant_idx ON restaurant_images (restaurant_id, id);

CREATE TABLE bookings (
  id              BIGSERIAL PRIMARY KEY,
  restaurant_id   BIGINT NOT NULL REFERENCES restaurants(id),
  user_id         BIGINT NOT NULL REFERENCES users(id),
  party_size      INT NOT NULL CHECK (party_size > 0),
  start_at        TIMESTAMPTZ NOT NULL,
  end_at          TIMESTAMPTZ NOT NULL,
  status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'cancelled')),
  cancel_deadline TIMESTAMPTZ NOT NULL,
  cancelled_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (end_at - start_at BETWEEN INTERVAL '30 minutes' AND INTERVAL '3 hours'),
  CHECK (cancel_deadline <= start_at - INTERVAL '30 minutes'),
  CHECK ((status = 'cancelled') = (cancelled_at IS NOT NULL))
);
CREATE INDEX bookings_restaurant_start_idx ON bookings (restaurant_id, start_at);
CREATE INDEX bookings_user_start_idx       ON bookings (user_id, start_at);

CREATE TABLE reviews (
  id            BIGSERIAL PRIMARY KEY,
  restaurant_id BIGINT NOT NULL REFERENCES restaurants(id),
  user_id       BIGINT NOT NULL REFERENCES users(id),
  rating        SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  comment       TEXT NOT NULL CHECK (char_length(btrim(comment)) BETWEEN 1 AND 1000),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, restaurant_id)
);
CREATE INDEX reviews_restaurant_idx ON reviews (restaurant_id);
