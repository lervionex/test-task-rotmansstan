CREATE TABLE IF NOT EXISTS accounts (
    user_id TEXT PRIMARY KEY,
    balance BIGINT NOT NULL CHECK (balance >= 0),
    currency TEXT NOT NULL CHECK (currency = 'USDT'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS withdrawals (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES accounts (user_id),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency TEXT NOT NULL CHECK (currency = 'USDT'),
    destination TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'confirmed')),
    idempotency_key TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at TIMESTAMPTZ NULL,
    UNIQUE (user_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    user_id TEXT NOT NULL REFERENCES accounts (user_id),
    idempotency_key TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    response_status INT NULL,
    response_body JSONB NULL,
    withdrawal_id UUID NULL REFERENCES withdrawals (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    withdrawal_id UUID NULL REFERENCES withdrawals (id),
    user_id TEXT NOT NULL REFERENCES accounts (user_id),
    entry_type TEXT NOT NULL,
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL CHECK (currency = 'USDT'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
