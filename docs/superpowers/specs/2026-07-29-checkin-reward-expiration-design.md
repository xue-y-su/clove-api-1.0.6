# Check-in Reward Expiration Design

## Goal

Add an optional policy that prevents daily check-in rewards from accumulating.
When the policy is enabled, a newly issued check-in reward is usable until the
next local midnight. Every real wallet debit consumes that reward first. Any
remaining reward is reclaimed after midnight without touching unrelated wallet
funds.

The policy is disabled by default and is controlled by an independent switch
inside the existing check-in settings section.

## Product Rules

1. The existing check-in feature and reward range continue to work unchanged.
2. `checkin_setting.expire_unused_quota` controls expiration and defaults to
   `false`.
3. The setting is captured when a reward is issued. Enabling or disabling the
   option does not change historical or already-issued rewards.
4. A reward issued while expiration is enabled expires at the next midnight in
   the server's configured local timezone.
5. Every negative wallet adjustment counts as use, including API billing, task
   billing, redemption reservations, transfers, and administrator deductions.
6. Wallet credits do not restore a consumed check-in reward. A refund or later
   credit is ordinary wallet balance.
7. Wallet debits consume the active check-in reward before any other wallet
   value. If the reward is fully consumed, expiration makes no wallet change.
8. Existing check-in rows are not made expirable during migration.
9. Expiration never creates debt. If legacy or out-of-band data makes the wallet
   balance smaller than the recorded remainder, the task reclaims only the
   available non-negative balance and settles the record.

## Data Model

Extend `checkins` with fields that describe the lifecycle of one issued reward:

- `quota_remaining BIGINT NOT NULL DEFAULT 0`: the reward amount not yet
  consumed by wallet debits.
- `expires_at BIGINT NOT NULL DEFAULT 0`: Unix timestamp of the next local
  midnight. Zero means this reward is permanent.
- `reclaimed_quota BIGINT NOT NULL DEFAULT 0`: the amount actually removed at
  expiration.
- `settled_at BIGINT NOT NULL DEFAULT 0`: timestamp at which an expirable reward
  was fully consumed or reclaimed. Zero means it is still pending.

The migration defaults make every existing row permanent. New rows populate the
lifecycle fields only when the expiration option is enabled.

Add an index on `(expires_at, settled_at)` for bounded due-record scans. The
existing unique index on `(user_id, checkin_date)` remains the issuance
idempotency boundary.

## Wallet Integration

`AdjustWalletWithTx` remains the single durable wallet mutation boundary. The
check-in lifecycle is integrated there so every supported debit has identical
behavior.

For a new negative wallet adjustment:

1. Lock the user wallet row.
2. Settle any overdue reward for that user before applying the debit. This
   closes the short interval between midnight and the background worker and
   prevents spending an already-expired reward.
3. Apply the requested wallet ledger entry and balance update.
4. Lock the user's current unexpired reward row and reduce
   `quota_remaining` by `min(abs(delta), quota_remaining)`.
5. If the remainder reaches zero, set `settled_at` to the adjustment timestamp.

All steps share one database transaction. Existing operation IDs continue to
make retries idempotent. An internal skip flag prevents the expiration debit
from recursively invoking expiration settlement.

Positive adjustments leave `quota_remaining` unchanged. This includes top-ups,
refunds, invitation rewards, and administrator credits.

## Expiration Flow

Add a master-node maintenance task following the existing subscription reset
task pattern:

- Run once at startup and then once per minute.
- Query due, unsettled check-ins in bounded batches.
- For each row, start a transaction and lock the user before the check-in row.
- Re-read the row after locking to handle races with a concurrent debit.
- Reclaim `min(quota_remaining, max(wallet_balance, 0))` through a wallet ledger
  entry with operation ID `checkin-expiry:<checkin_id>` and category
  `checkin_expiry`.
- Store the actual reclaimed amount, set the remainder to zero, and set
  `settled_at` in the same transaction.
- Invalidate the user cache only after the transaction commits.

User row then check-in row is the lock order for both the request path and the
worker. This prevents worker/debit deadlocks. The stable operation ID and the
settled marker make startup retries and multi-node races harmless.

Failures are logged once per failed batch and retried on the next tick. Routine
successful scans do not add system log noise. The canonical wallet adjustment
audit remains available for actual reclaimed amounts.

## Settings UI

Extend the existing **Check-in Rewards** settings section with a switch shown
when check-in is enabled:

- Label: `Expire unused check-in rewards daily`
- Description: `New check-in rewards expire at the next midnight. Wallet
  spending uses the reward first.`

The form saves `checkin_setting.expire_unused_quota` through the existing option
mutation API. Older servers and saved configurations default the field to
`false`.

The user check-in response and monthly record projection expose
`quota_remaining`, `expires_at`, and `reclaimed_quota`. The profile calendar can
show an expiration note for today's expirable reward without adding a new
workflow or modal.

## Compatibility

- Default behavior remains permanent check-in rewards.
- Existing database rows remain permanent after migration.
- MySQL, PostgreSQL, and SQLite use the same business rules.
- Only the master node starts the periodic worker.
- The implementation uses the existing wallet ledger, cache invalidation,
  global option registry, settings forms, and translation system.

## Testing

Backend tests cover:

- fixed and random reward issuance with the option off and on;
- next-midnight calculation in a supplied timezone;
- partial debit, full debit, and debit larger than the reward;
- debit categories including relay, redemption, and administrator adjustment;
- credits and refunds not restoring a consumed reward;
- no-op expiration after full use;
- partial and full reclamation;
- balance protection when stored balance is lower than the remainder;
- stable operation IDs and repeated worker execution;
- midnight debit settlement before the requested debit;
- concurrent debit and expiration lock ordering;
- existing rows remaining permanent.

Frontend tests cover:

- settings typing and default hydration;
- rendering and saving the independent switch;
- all supported locale dictionaries containing the new labels;
- the profile notice appearing only for an expirable current reward.

The final verification includes Go tests, the complete default frontend test
suite, type/build checks, formatting, lint, a local MySQL upgrade using the
production-copy database, and desktop/mobile browser checks of both the admin
setting and user check-in card.

## Rollout And Rollback

Before production rollout, take a fresh database dump and preserve the current
Compose file and image. Deploy with the option still disabled, verify schema
migration and ordinary check-in behavior, then enable the option explicitly in
system settings.

Application rollback is safe while the new columns remain in the database; the
previous version ignores them. Disabling the setting affects only rewards issued
after the change, so already-issued expirable rewards retain their documented
expiry contract.
