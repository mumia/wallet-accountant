# Households

Who uses Wallet Accountant and what they share. A Household is the only ownership and privacy boundary (ADR-0003).

## Language

**User**:
A person who uses Wallet Accountant. A User exists only after redeeming an Invitation, and belongs to one or more Households.
_Avoid_: Customer, member account

**Invitation**:
A single-use, expiring code that admits one person to the instance as a User when they redeem it after signing in. The first one is issued by the operator with the `bootstrap` run mode (ADR-0017).
_Avoid_: Sign-up link, registration token

**Instance administrator**:
A User who controls who is admitted to the instance. It grants no access to any Household's data.
_Avoid_: Admin, superuser, owner

**Unregistered**:
Someone the OIDC provider has authenticated who has not redeemed an Invitation. They can see their registration status and redeem an Invitation, nothing else.
_Avoid_: Pending user, guest

**Household**:
A group of one or more Users who share all of its Accounts, Categories and Budgets. Everything in a Household is visible to all its members; money that should stay private belongs in a separate Household.
_Avoid_: Family, group, team
