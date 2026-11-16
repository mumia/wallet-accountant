-- Runs once, when the development PostgreSQL volume is created.
-- Development credentials only; never use them anywhere else.
--
-- Creates wa_owner, which owns the database and runs the migrations
-- (ADR-0014). The first migration creates wa_app and wa_event_store_reader;
-- creating a BYPASSRLS role needs CREATEROLE and BYPASSRLS. `make migrate`
-- then sets their passwords to those in WA_DATABASE_URL and
-- WA_READER_DATABASE_URL.

CREATE ROLE wa_owner LOGIN CREATEROLE BYPASSRLS PASSWORD 'wa_owner';
CREATE DATABASE wallet_accountant OWNER wa_owner;
