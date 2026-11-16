#!/bin/sh
# Runs once, when the PostgreSQL volume is created (compose.yaml).
#
# Creates wa_owner, which owns the database and runs the migrations
# (ADR-0014). The first migration creates wa_app and wa_event_store_reader;
# the migrate run mode then sets their passwords to those in WA_DATABASE_URL
# and WA_READER_DATABASE_URL.
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
	-v owner_password="$WA_OWNER_PASSWORD" <<'SQL'
CREATE ROLE wa_owner LOGIN CREATEROLE BYPASSRLS PASSWORD :'owner_password';
CREATE DATABASE wallet_accountant OWNER wa_owner;
SQL
