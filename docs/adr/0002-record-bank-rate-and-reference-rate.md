# Record both the Bank rate and the Reference rate on every conversion

When money changes currency, we store the rate the bank actually applied (the Bank rate) as the truth for balances. Alongside it we store a published daily Reference rate for the transaction date. The Bank rate is the only one that reconciles with statements. The Reference rate lets us analyse what conversions really cost (the bank's margin) and compare across Households and Accounts consistently. Both are stored when the Transaction is recorded, not looked up later, so historical reports never change after the fact.
