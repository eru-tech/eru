-- OAuth 2.0 client registry for eru-auth's own authorization server.
-- Applies when an auth is configured with "oauth_server": {"backend": "ERU"}. With the default
-- HYDRA backend the clients live in hydra and this table is unused.

create table if not exists eruauth_oauth_clients (
    client_id                  text        not null,
    project_id                 text        not null,
    auth_name                  text        not null,
    client_name                text,
    -- Secrets are stored only as a sha512 hex digest. A public client (token_endpoint_auth_method
    -- 'none') holds no secret at all and is authenticated by pkce.
    client_secret_hash         text,
    token_endpoint_auth_method text        not null default 'none',
    redirect_uris              jsonb       not null default '[]'::jsonb,
    grant_types                jsonb       not null default '[]'::jsonb,
    response_types             jsonb       not null default '["code"]'::jsonb,
    scope                      text,
    created_date               timestamptz not null default now(),
    updated_date               timestamptz not null default now(),
    constraint eruauth_oauth_clients_pk primary key (project_id, auth_name, client_id)
);

-- Client ids are uuids, but the primary key is scoped by project and auth, so a lookup by id alone
-- still needs its own index for the token endpoint's hot path.
create index if not exists eruauth_oauth_clients_client_id
    on eruauth_oauth_clients (client_id);

-- One authorization in flight: from /oauth2/auth, through login and consent, to the issued code.
-- Only used with the ERU backend. eruauth_pkce_events and eruauth_temp_codes are left alone - they
-- serve the Microsoft and Google SSO flows, where eru-auth is the oauth client rather than the
-- server, and eruauth_temp_codes is keyed one row per identity which cannot hold concurrent grants.

create table if not exists eruauth_oauth_auth_requests (
    -- The request id and the code are handed to the browser and the client; only their digests are
    -- stored, so a leaked row cannot be used to resume or redeem someone else's authorization.
    request_hash          text        not null primary key,
    project_id            text        not null,
    auth_name             text        not null,
    client_id             text        not null,
    redirect_uri          text        not null,
    requested_scope       text,
    granted_scope         text,
    state                 text,
    nonce                 text,
    code_challenge        text,
    code_challenge_method text,
    identity_id           text,
    code_hash             text,
    -- Set when the code is redeemed rather than deleting the row, so a replayed code is detected
    -- rather than merely absent.
    consumed_at           timestamptz,
    expires_at            timestamptz not null,
    created_date          timestamptz not null default now()
);

create index if not exists eruauth_oauth_auth_requests_code
    on eruauth_oauth_auth_requests (code_hash);

create index if not exists eruauth_oauth_auth_requests_expiry
    on eruauth_oauth_auth_requests (expires_at);

-- Refresh tokens for the ERU backend. grant_id is the authorization request the family descends
-- from, so a reused token can revoke every token issued from the same grant.
create table if not exists eruauth_oauth_refresh_tokens (
    token_hash   text        not null primary key,
    grant_id     text        not null,
    project_id   text        not null,
    auth_name    text        not null,
    client_id    text        not null,
    identity_id  text        not null,
    scope        text,
    -- Set when the token is exchanged for its successor. A rotated token presented again is a
    -- replay, not a mistake, and revokes the whole family.
    rotated_at   timestamptz,
    revoked_at   timestamptz,
    expires_at   timestamptz not null,
    created_date timestamptz not null default now()
);

create index if not exists eruauth_oauth_refresh_tokens_grant
    on eruauth_oauth_refresh_tokens (grant_id);

create index if not exists eruauth_oauth_refresh_tokens_expiry
    on eruauth_oauth_refresh_tokens (expires_at);

-- Browser sessions for the ERU backend: what lets a second authorization skip the login screen.
-- Only the digest of the cookie value is stored, so the cookie cannot be reconstructed from a row.
create table if not exists eruauth_oauth_sessions (
    session_hash text        not null primary key,
    project_id   text        not null,
    auth_name    text        not null,
    identity_id  text        not null,
    revoked_at   timestamptz,
    expires_at   timestamptz not null,
    created_date timestamptz not null default now()
);

create index if not exists eruauth_oauth_sessions_identity
    on eruauth_oauth_sessions (project_id, auth_name, identity_id);

create index if not exists eruauth_oauth_sessions_expiry
    on eruauth_oauth_sessions (expires_at);

-- Remembered consent: what a user has already agreed to give a client, so an authorization for the
-- same scope is not shown again. Revoking a row makes the next authorization ask afresh.
create table if not exists eruauth_oauth_consents (
    project_id    text        not null,
    auth_name     text        not null,
    client_id     text        not null,
    identity_id   text        not null,
    granted_scope text,
    revoked_at    timestamptz,
    created_date  timestamptz not null default now(),
    updated_date  timestamptz not null default now(),
    constraint eruauth_oauth_consents_pk primary key (project_id, auth_name, client_id, identity_id)
);

create index if not exists eruauth_oauth_consents_identity
    on eruauth_oauth_consents (project_id, auth_name, identity_id);
