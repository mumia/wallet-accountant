# 10: Recategorise, reverse Transactions and Discard pending Imported transactions

**What to build:** a User gives an Uncategorised Transaction its Category or changes a wrong one. A mistaken Accept is reversed: a linked reversing Transaction appears in the list beside the original, and the Imported transaction goes back to pending, where it can be Accepted again or Discarded. Spec: `../spec.md` stories 50–51, 51b–51f, "Ledger: Transaction", "Cross-context calls: Reverse Transaction"; ADR-0018.

**Blocked by:** 09 (Accept Imported transactions)

**Status:** claimed

- [ ] `PATCH …/transactions/{transaction_id}` sets the Category of the non-Account side and emits an event; the Uncategorised count updates
- [ ] `POST …/transactions/{transaction_id}/reverse` emits `reversed` and records a reversing Transaction with opposite Postings, the same Booking date and a link to the original; both appear in the Account's list
- [ ] Reversing a Transaction linked to an Imported transaction returns it to pending through an Ingestion `app` port in the same database transaction
- [ ] Refused: reversing the Opening balance Transaction, a reversal, or a Transaction already reversed; recategorising a reversed Transaction or a reversal
- [ ] A reversed Transaction and its reversal don't count toward the Uncategorised count
- [ ] `POST …/imported-transactions/{imported_transaction_id}/discard` Discards a pending Imported transaction; Discarded is final
- [ ] Decider tests: reversal rules, recategorisation rules, pending → Discarded
- [ ] HTTP tests: recategorise, reverse then re-Accept, reverse then Discard, each refusal; another Household gets 404
- [ ] Consistency map rows for Set Transaction Category, Reverse Transaction and Discard match the code
