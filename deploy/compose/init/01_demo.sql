-- Optional demo tables for NL2SQL smoke tests (runs on first Postgres init only).
CREATE TABLE IF NOT EXISTS demo_sales (
    id SERIAL PRIMARY KEY,
    region TEXT NOT NULL,
    amount_cents INT NOT NULL,
    sold_on DATE NOT NULL
);

INSERT INTO demo_sales (region, amount_cents, sold_on) VALUES
    ('east', 12000, '2026-01-10'),
    ('west', 8500, '2026-01-12'),
    ('east', 9900, '2026-02-01');
