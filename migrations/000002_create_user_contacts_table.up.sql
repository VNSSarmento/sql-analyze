CREATE TABLE user_contacts (
    id BIGSERIAL PRIMARY KEY,
    db_user TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL,
    display_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);