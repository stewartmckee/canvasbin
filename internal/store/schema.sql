CREATE TABLE IF NOT EXISTS pages (
    id             bigserial PRIMARY KEY,
    title          text        NOT NULL DEFAULT '',
    format         text        NOT NULL DEFAULT 'html' CHECK (format IN ('html', 'markdown')),
    content        text        NOT NULL DEFAULT '',
    read_key_hash  bytea       NOT NULL UNIQUE,
    write_key_hash bytea       NOT NULL UNIQUE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
