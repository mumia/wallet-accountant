# Shared kernel

Language that every bounded context uses in the same way: currencies, exchange rates and Matching.

## Language

**Base currency**:
The currency a Household reports and budgets in. Amounts in other currencies are converted into it. A Household can change its Base currency.
_Avoid_: Home currency, main currency

**Bank rate**:
The exchange rate the bank actually applied when it converted a payment into an Account currency.
_Avoid_: Real rate, actual rate

**Reference rate**:
A published daily exchange rate for the transaction date, recorded alongside the Bank rate for analysis.
_Avoid_: Market rate, official rate

**Match**:
A confirmed link between two records of the same money movement: a manually entered transaction and the matching Imported transaction, a Scheduled transaction occurrence and the real transaction, the two Transfer legs of a Transfer, or the two sides of a Household transfer. The system suggests a Match and a User confirms it.
_Avoid_: Duplicate, merge, reconciliation
