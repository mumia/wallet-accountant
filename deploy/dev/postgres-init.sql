-- Runs once, when the development PostgreSQL volume is created.
-- Development credentials only; never use them anywhere else.

CREATE ROLE wallet_accountant LOGIN PASSWORD 'wallet_accountant';
CREATE DATABASE wallet_accountant OWNER wallet_accountant;
