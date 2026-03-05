INSERT INTO accounts (user_id, balance, currency)
VALUES ('user-1', 1000, 'USDT')
ON CONFLICT (user_id) DO NOTHING;
