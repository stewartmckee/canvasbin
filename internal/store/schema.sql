-- The whole file runs as one implicit transaction; serialise concurrent starts.
SELECT pg_advisory_xact_lock(4242001);

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

ALTER TABLE pages ADD COLUMN IF NOT EXISTS owner_id bigint;
-- Only set for owned pages: lets an owner's library link back to a page. The
-- read key is a read-only bearer link, so retaining it is a deliberate tradeoff.
ALTER TABLE pages ADD COLUMN IF NOT EXISTS read_key text;

CREATE TABLE IF NOT EXISTS users (
    id                     bigserial PRIMARY KEY,
    email                  text        NOT NULL UNIQUE,
    password_hash          text        NOT NULL,
    display_name           text        NOT NULL DEFAULT '',
    email_verified_at      timestamptz,
    disabled_at            timestamptz,
    stripe_customer_id     text UNIQUE,
    stripe_subscription_id text UNIQUE,
    stripe_price_id        text,
    subscription_status    text        NOT NULL DEFAULT '',
    current_period_end     timestamptz,
    cancel_at_period_end   boolean     NOT NULL DEFAULT false,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'pages_owner_id_fkey') THEN
        ALTER TABLE pages
            ADD CONSTRAINT pages_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS pages_owner_id_idx ON pages (owner_id) WHERE owner_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS sessions (
    id         bigserial PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS email_tokens (
    id         bigserial PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose    text        NOT NULL CHECK (purpose IN ('verify', 'reset')),
    token_hash bytea       NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS page_shares (
    page_id    bigint      NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    user_id    bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       text        NOT NULL CHECK (role IN ('viewer', 'editor')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (page_id, user_id)
);

CREATE TABLE IF NOT EXISTS page_invites (
    id         bigserial PRIMARY KEY,
    page_id    bigint      NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    email      text        NOT NULL,
    role       text        NOT NULL CHECK (role IN ('viewer', 'editor')),
    invited_by bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (page_id, email)
);

-- Teams: a shared space with its own subscription (base price plus one seat
-- per member).
CREATE TABLE IF NOT EXISTS teams (
    id                     bigserial PRIMARY KEY,
    name                   text        NOT NULL,
    created_by             bigint      REFERENCES users(id) ON DELETE SET NULL,
    stripe_customer_id     text UNIQUE,
    stripe_subscription_id text UNIQUE,
    subscription_status    text        NOT NULL DEFAULT '',
    current_period_end     timestamptz,
    cancel_at_period_end   boolean     NOT NULL DEFAULT false,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS team_members (
    team_id    bigint      NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id    bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       text        NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX IF NOT EXISTS team_members_user_id_idx ON team_members (user_id);

CREATE TABLE IF NOT EXISTS team_invites (
    id         bigserial PRIMARY KEY,
    team_id    bigint      NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    email      text        NOT NULL,
    role       text        NOT NULL CHECK (role IN ('admin', 'member')),
    invited_by bigint      REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (team_id, email)
);

ALTER TABLE pages ADD COLUMN IF NOT EXISTS team_id bigint;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'pages_team_id_fkey') THEN
        ALTER TABLE pages
            ADD CONSTRAINT pages_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS pages_team_id_idx ON pages (team_id) WHERE team_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS page_comments (
    id         bigserial PRIMARY KEY,
    page_id    bigint      NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    user_id    bigint      REFERENCES users(id) ON DELETE SET NULL,
    body       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS page_comments_page_id_idx ON page_comments (page_id, created_at);

-- Team timeline. page_title is copied so entries for deleted pages still read
-- sensibly.
CREATE TABLE IF NOT EXISTS team_activity (
    id         bigserial PRIMARY KEY,
    team_id    bigint      NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id    bigint      REFERENCES users(id) ON DELETE SET NULL,
    page_id    bigint      REFERENCES pages(id) ON DELETE SET NULL,
    page_title text        NOT NULL DEFAULT '',
    action     text        NOT NULL,
    detail     text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS team_activity_team_idx ON team_activity (team_id, created_at DESC);
CREATE INDEX IF NOT EXISTS team_activity_page_idx ON team_activity (page_id, created_at DESC) WHERE page_id IS NOT NULL;

-- OAuth for MCP
CREATE TABLE IF NOT EXISTS oauth_clients (
    client_id                  text PRIMARY KEY,
    client_secret_hash         bytea,
    redirect_uris              text[]      NOT NULL,
    client_name                text        NOT NULL DEFAULT '',
    token_endpoint_auth_method text        NOT NULL DEFAULT 'none',
    created_at                 timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS oauth_auth_codes (
    code_hash             bytea PRIMARY KEY,
    client_id             text        NOT NULL,
    user_id               bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri          text        NOT NULL,
    code_challenge        text        NOT NULL,
    code_challenge_method text        NOT NULL DEFAULT 'S256',
    scope                 text        NOT NULL,
    resource              text        NOT NULL,
    expires_at            timestamptz NOT NULL,
    used_at               timestamptz
);

CREATE TABLE IF NOT EXISTS oauth_tokens (
    id                 bigserial PRIMARY KEY,
    access_hash        bytea       NOT NULL UNIQUE,
    refresh_hash       bytea UNIQUE,
    client_id          text        NOT NULL,
    user_id            bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope              text        NOT NULL,
    resource           text        NOT NULL,
    access_expires_at  timestamptz NOT NULL,
    refresh_expires_at timestamptz,
    revoked_at         timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now()
);

-- Live reload: tell listeners when a page's visible content changes or it is
-- deleted. Ownership and sharing changes do not notify.
CREATE OR REPLACE FUNCTION notify_page_change() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM pg_notify('page_changed', json_build_object('id', OLD.id, 'deleted', true)::text);
        RETURN OLD;
    END IF;
    IF NEW.title IS DISTINCT FROM OLD.title
       OR NEW.format IS DISTINCT FROM OLD.format
       OR NEW.content IS DISTINCT FROM OLD.content THEN
        PERFORM pg_notify('page_changed', json_build_object('id', NEW.id, 'updated_at', NEW.updated_at)::text);
    END IF;
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER pages_notify_change
    AFTER UPDATE OR DELETE ON pages
    FOR EACH ROW EXECUTE FUNCTION notify_page_change();
