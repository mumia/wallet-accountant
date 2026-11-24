# Households

Who uses Wallet Accountant and what they share. A Household is the only ownership and privacy boundary (ADR-0003).

## Language

**User**:
A person who uses Wallet Accountant, known to others by a display name they choose. A User exists only after redeeming an Invitation, and belongs to any number of Households.
_Avoid_: Customer, member account

**Invitation**:
A single-use, expiring code that admits one person to the instance, to a Household, or to both when they redeem it after signing in. The first one is issued by the operator with the `bootstrap` run mode (ADR-0017); every later one is issued by a Member to their Household (ADR-0025).
_Avoid_: Sign-up link, registration token

**Instance administrator**:
A User who controls who is admitted to the instance. It grants no access to any Household's data.
_Avoid_: Admin, superuser, owner

**Unregistered**:
Someone the OIDC provider has authenticated who has not redeemed an Invitation. They can see their registration status and redeem an Invitation, nothing else.
_Avoid_: Pending user, guest

**Household**:
A group of Members who share all of its Accounts, Categories and Budgets. It always has at least one Member. Everything in a Household is visible to all its Members; money that should stay private belongs in a separate Household.
_Avoid_: Family, group, team

**Member**:
A User who belongs to a Household. Every Member is equal and sees everything in it.
_Avoid_: Participant, owner
