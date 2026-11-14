# Money is an exact decimal plus a currency, never an integer or a float

Every amount is a pair: an exact decimal value and a currency code. It is stored as `NUMERIC` in PostgreSQL and as a decimal type in Go. Exchange rates (Bank rate and Reference rate) are exact decimals too. Each currency defines how many decimal places it uses, for example EUR 2, JPY 0, BTC 8 and ETH 18. We rejected integer minor units (cents) because 18-decimal crypto amounts overflow 64-bit integers and exchange rates need more precision than any currency's minor unit. Floats are never acceptable for money.
