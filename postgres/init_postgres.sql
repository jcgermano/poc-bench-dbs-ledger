CREATE TABLE IF NOT EXISTS accounts (
    id BIGINT PRIMARY KEY,
    user_data BIGINT,
    debits_pending BIGINT DEFAULT 0,
    debits_posted BIGINT DEFAULT 0,
    credits_pending BIGINT DEFAULT 0,
    credits_posted BIGINT DEFAULT 0,
    timestamp BIGINT
);

CREATE TABLE IF NOT EXISTS transfers (
    id BIGINT PRIMARY KEY,
    debit_account_id BIGINT NOT NULL,
    credit_account_id BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    timestamp BIGINT,
    user_data BIGINT
);
