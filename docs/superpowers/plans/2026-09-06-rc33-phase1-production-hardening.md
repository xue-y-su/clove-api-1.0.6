# rc33 Phase 1 Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Selectively integrate the low-coupling production fixes from New API rc33 into Clove API without changing Clove wallet, subscription, token, group, redemption, invitation, or audit semantics.

**Architecture:** Keep Clove's current application and web/default boundaries. Port each upstream fix behind a focused test, run PostgreSQL migration tests against the local Docker database, and preserve the existing five-minute relay header timeout as the configurable default. Back up and restore the local database before any migrated build is allowed to start.

**Tech Stack:** Go 1.24, GORM 1.25.12, PostgreSQL 15, modernc SQLite through github.com/glebarez/sqlite, Bun, React/TypeScript, Docker Compose.

---

### Task 1: Capture and prove the local PostgreSQL rollback point

**Files:**
- Create: .codex-tmp/rc33-phase1-backup/new-api-pre-rc33.dump
- Create: .codex-tmp/rc33-phase1-backup/pre-counts.tsv
- Create: .codex-tmp/rc33-phase1-backup/pre-schema.sql
- Create: .codex-tmp/rc33-phase1-backup/restore-counts.tsv

- [x] **Step 1: Create the data-disk backup directory**

Run:

~~~powershell
New-Item -ItemType Directory -Force .codex-tmp/rc33-phase1-backup | Out-Null
~~~

Expected: the directory exists under the D: workspace.

- [x] **Step 2: Record table counts and schema**

Run:

~~~powershell
docker exec new-api-dev-pg sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -AtF "|" -c "SELECT relname,n_live_tup FROM pg_stat_user_tables ORDER BY relname"' | Set-Content -Encoding utf8 .codex-tmp/rc33-phase1-backup/pre-counts.tsv
docker exec new-api-dev-pg sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --schema-only --no-owner --no-privileges' | Set-Content -Encoding utf8 .codex-tmp/rc33-phase1-backup/pre-schema.sql
~~~

Expected: both files are non-empty and include users, tokens, channels, and options.

- [x] **Step 3: Create and inspect a custom-format logical dump**

Run:

~~~powershell
docker exec new-api-dev-pg sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges -f /tmp/new-api-pre-rc33.dump'
docker cp new-api-dev-pg:/tmp/new-api-pre-rc33.dump .codex-tmp/rc33-phase1-backup/new-api-pre-rc33.dump
docker exec new-api-dev-pg rm -f /tmp/new-api-pre-rc33.dump
docker run --rm -v "$(Get-Location)/.codex-tmp/rc33-phase1-backup:/backup:ro" postgres:15 pg_restore --list /backup/new-api-pre-rc33.dump
~~~

Expected: pg_restore lists the dump without an error.

- [x] **Step 4: Restore into an isolated database**

Run:

~~~powershell
docker cp .codex-tmp/rc33-phase1-backup/new-api-pre-rc33.dump new-api-dev-pg:/tmp/new-api-pre-rc33.dump
docker exec new-api-dev-pg sh -lc 'dropdb -U "$POSTGRES_USER" --if-exists clove_rc33_restore_test && createdb -U "$POSTGRES_USER" clove_rc33_restore_test && pg_restore -U "$POSTGRES_USER" -d clove_rc33_restore_test --clean --if-exists --no-owner --no-privileges /tmp/new-api-pre-rc33.dump'
docker exec new-api-dev-pg sh -lc 'psql -U "$POSTGRES_USER" -d clove_rc33_restore_test -AtF "|" -c "SELECT relname,n_live_tup FROM pg_stat_user_tables ORDER BY relname"' | Set-Content -Encoding utf8 .codex-tmp/rc33-phase1-backup/restore-counts.tsv
~~~

Expected: restore exits zero and the restored database has the same application tables.

### Task 2: Enable effective SQLite writer serialization

**Files:**
- Modify: common/database.go
- Create: common/database_test.go

- [x] **Step 1: Write the failing SQLite runtime test**

Add:

~~~go
func TestSQLitePathEnablesConcurrentWritePragmas(t *testing.T) {
	dsn := filepath.ToSlash(filepath.Join(t.TempDir(), "test.db")) +
		strings.TrimPrefix(SQLitePath, "one-api.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	var journalMode string
	require.NoError(t, db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error)
	assert.Equal(t, "wal", strings.ToLower(journalMode))

	var busyTimeout int
	require.NoError(t, db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error)
	assert.Equal(t, 30000, busyTimeout)
}
~~~

- [x] **Step 2: Run the test and verify RED**

Run:

~~~powershell
go test ./common -run TestSQLitePathEnablesConcurrentWritePragmas -count=1
~~~

Expected: FAIL because the existing DSN keeps delete journal mode and the driver's default timeout.

- [x] **Step 3: Update the SQLite DSN**

Set:

~~~go
var SQLitePath = "one-api.db?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_txlock=immediate"
~~~

- [x] **Step 4: Run the focused and package tests**

Run:

~~~powershell
go test ./common -run TestSQLitePathEnablesConcurrentWritePragmas -count=1
go test ./common -count=1
~~~

Expected: PASS.

### Task 3: Make PostgreSQL transaction-pool compatible

**Files:**
- Modify: go.mod
- Modify: go.sum
- Modify: model/channel.go
- Modify: model/main.go
- Modify: model/gorm_logger.go
- Modify: model/gorm_logger_test.go
- Modify: model/prefill_group.go
- Modify: model/task.go
- Create: model/json_column_test.go
- Create: model/postgres_config_test.go

- [x] **Step 1: Write failing configuration and diagnostic tests**

Add:

~~~go
func TestPostgresGormConfigDisablesPreparedStatements(t *testing.T) {
	assert.False(t, newPostgresGormConfig().PrepareStmt)
}
~~~

Extend TestSanitizeDBErrorStripsDriverMessage with 08P01 and 42P05 cases. Each case must assert that the secret driver message is absent and the stable remediation text is present.

- [x] **Step 2: Run the tests and verify RED**

Run:

~~~powershell
go test ./model -run 'TestPostgresGormConfigDisablesPreparedStatements|TestSanitizeDBErrorStripsDriverMessage' -count=1
~~~

Expected: FAIL because newPostgresGormConfig does not exist and the conflict errors lack remediation text.

- [x] **Step 3: Implement the PostgreSQL-only configuration**

Add:

~~~go
func newPostgresGormConfig() *gorm.Config {
	return newGormConfig(false)
}
~~~

Use it only in the PostgreSQL branch of chooseDB. Keep newGormConfig(true) for SQLite and MySQL. Add the sanitized 08P01/42P05 remediation from upstream commit 66031a09.

- [x] **Step 4: Upgrade GORM and the PostgreSQL driver to the upstream-tested pair**

Run:

~~~powershell
go get gorm.io/gorm@v1.25.12
go get gorm.io/driver/postgres@v1.5.9
go mod tidy
~~~

Expected: go.mod pins gorm.io/gorm v1.25.12 and gorm.io/driver/postgres v1.5.9.

- [x] **Step 5: Make JSON column Valuers compatible with pgx simple protocol**

The PostgreSQL integration test reproduced SQLSTATE 22P02 with byte-slice Valuers. Port the rc33 JSON contract tests and make ChannelInfo, Properties, TaskPrivateData, and JSONValue return string or nil. Normalize Scanner input through jsonScanBytes so both string and byte-slice results remain readable.

- [x] **Step 6: Run model tests**

Run:

~~~powershell
go test ./model -run 'TestPostgresGormConfigDisablesPreparedStatements|TestSanitizeDBErrorStripsDriverMessage' -count=1
go test ./model -count=1
~~~

Expected: PASS.

### Task 4: Migrate only the known Prefill Group legacy constraint

**Files:**
- Create: model/prefill_group_migration.go
- Create: model/prefill_group_migration_test.go
- Modify: model/main.go

- [x] **Step 1: Port the upstream migration tests before implementation**

Port the rc33 cases from model/prefill_group_migration_test.go. Preserve these assertions:

~~~go
assert.Zero(t, globalConstraintCount)
assert.Zero(t, globalIndexCount)
assert.Contains(t, strings.ToLower(targetIndexDefinition), "where (deleted_at is null)")
~~~

Cases: fresh PostgreSQL, known legacy constraint, known legacy standalone index, unknown constraint, unknown index, preserved non-conflicting indexes, repeated execution, SQLite, and optional MySQL.

- [x] **Step 2: Run the tests and verify RED**

Run:

~~~powershell
$env:TEST_POSTGRES_DSN='postgresql://root:123456@127.0.0.1:5432/new-api?sslmode=disable'
go test ./model -run TestMigratePrefillGroupUniqueness -count=1
~~~

Expected: FAIL because migratePrefillGroupUniqueness and its constants do not exist.

- [x] **Step 3: Port the scoped migration implementation**

Port model/prefill_group_migration.go from upstream commit 2b6f1dfe. Call it at the beginning of migrateDB, before AutoMigrate:

~~~go
func migrateDB() error {
	if err := migratePrefillGroupUniqueness(DB); err != nil {
		return err
	}
~~~

Keep Clove's migrateDBFast and its model list unchanged.

- [x] **Step 4: Run PostgreSQL and SQLite migration tests**

Run:

~~~powershell
$env:TEST_POSTGRES_DSN='postgresql://root:123456@127.0.0.1:5432/new-api?sslmode=disable'
go test ./model -run TestMigratePrefillGroupUniqueness -count=1
go test ./model -run TestMainDatabaseMigrations -count=1
~~~

Expected: PASS. The MySQL case may report SKIP when TEST_MYSQL_DSN is absent.

### Task 5: Preserve quota while calculating RPM and TPM

**Files:**
- Modify: model/log.go
- Verify: model/log_ip_admin_test.go

- [x] **Step 1: Use the existing failing regression tests**

Create two recent consume logs in an isolated SQLite database, then assert:

~~~go
stat, err := SumUsedQuotaWithIP(
	LogTypeConsume, now-3600, now+1, "", "", "", "", 0, "",
)
require.NoError(t, err)
assert.Equal(t, 300, stat.Quota)
assert.Equal(t, 2, stat.Rpm)
assert.Equal(t, 34, stat.Tpm)
~~~

- [x] **Step 2: Run the tests and verify RED**

Run:

~~~powershell
go test ./model -run TestSumUsedQuotaPreservesQuotaWhenScanningRates -count=1
~~~

Observed: both existing IP-filtered statistics tests failed with expected quota 11/13 and actual quota zero.

- [x] **Step 3: Scan RPM/TPM into an isolated projection**

Implement:

~~~go
var rateStat struct {
	Rpm int
	Tpm int
}
if err := rpmTpmQuery.Scan(&rateStat).Error; err != nil {
	common.SysError("failed to query rpm/tpm stat: " + err.Error())
	return stat, errors.New("查询统计数据失败")
}
stat.Rpm = rateStat.Rpm
stat.Tpm = rateStat.Tpm
~~~

- [x] **Step 4: Run the focused regression tests**

Run:

~~~powershell
go test ./model -run 'TestAdminLogQueryAndStatsFilterByIP|TestAdminLogQueryNormalizesLegacyLoopbackIPDisplayAndFilter' -count=1
~~~

Observed: PASS. The full model package will be rerun after the remaining model migration changes.

### Task 6: Correct time-range billing expressions in web/default

**Files:**
- Modify: web/default/src/features/pricing/lib/billing-expr.ts
- Create: web/default/src/features/pricing/lib/time-rule-expr.test.ts

- [x] **Step 1: Add the upstream regression suite using Bun imports**

Port rc33 time-rule-expr.test.ts and change its runner import to:

~~~typescript
import { describe, expect, test } from 'bun:test'
~~~

Keep cases for day ranges, overnight ranges, equal bounds, invalid values, parsing, mixed groups, and build-parse-build stability.

- [x] **Step 2: Run the test and verify RED**

Run:

~~~powershell
Set-Location web/default
bun test src/features/pricing/lib/time-rule-expr.test.ts
~~~

Expected: FAIL because 9..12 currently builds a logical-OR tautology and the parser splits logical-AND bounds.

- [x] **Step 3: Port the rc33 parser and builder correction**

Add TIME_FUNC_RANGES, isTimeValueInRange, tryParseTimeRangePair, whole-range parsing, adjacent-bound merging, and the start-less-than-or-equal logical-AND rule from upstream commit ac381acf. Keep Clove's existing public types and translation keys.

- [x] **Step 4: Run pricing tests**

Run:

~~~powershell
bun test src/features/pricing
bun test ./src ./rsbuild.config.test.ts
bun run typecheck
bun run build
~~~

Observed: the focused time-rule suite passed 32/32 and the complete pricing suite passed 47/47. Full frontend checks remain in Task 8.

### Task 7: Make relay response-header timeout configurable

**Files:**
- Modify: common/constants.go
- Modify: common/init.go
- Modify: service/http_client.go
- Modify: service/protected_fetch_client.go
- Modify: service/http_client_test.go
- Modify: .env.example

- [x] **Step 1: Write failing timeout behavior tests**

Replace fixed-constant assertions with table-driven cases for 0, 300, and an overflowing positive value. Set common.RelayResponseHeaderTimeout before building direct and protected transports and assert each effective ResponseHeaderTimeout.

- [x] **Step 2: Run the tests and verify RED**

Run:

~~~powershell
go test ./service -run 'TestRelayHTTPTransportsBoundConnectionAndResponseHeaders|TestRelayResponseHeaderTimeout' -count=1
~~~

Expected: FAIL because the environment-backed variable and conversion helper do not exist.

- [x] **Step 3: Implement one bounded conversion**

Add RelayResponseHeaderTimeout to common, load a default of 300 in InitEnv, and use this helper for direct and protected transports:

~~~go
func relayResponseHeaderTimeout() time.Duration {
	seconds := common.RelayResponseHeaderTimeout
	if seconds <= 0 {
		return 0
	}
	if seconds > maxTimeoutSeconds {
		seconds = maxTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}
~~~

Remove the fixed constant with the same name. Document RELAY_RESPONSE_HEADER_TIMEOUT=300 and that zero disables only the response-header wait limit.

- [x] **Step 4: Run transport tests**

Run:

~~~powershell
go test ./service -run 'TestRelayHTTPTransportsBoundConnectionAndResponseHeaders|TestRelayResponseHeaderTimeout|TestNewProxyHTTPClientKeepsRelayTransportPolicy' -count=1
go test ./service -count=1
~~~

Expected: PASS.

### Task 8: Rebuild, migrate, compare data, and run end-to-end regression

**Files:**
- Create: .codex-tmp/rc33-phase1-backup/post-counts.tsv
- Create: .codex-tmp/rc33-phase1-backup/post-schema.sql
- Create: .codex-tmp/rc33-phase1-backup/verification.md
- Modify only after a reproduced failure: the failing implementation and its regression test

- [ ] **Step 1: Run repository checks before rebuilding**

Run:

~~~powershell
go test ./... -count=1
go vet ./...
Set-Location web/default
bun test ./src ./rsbuild.config.test.ts
bun run typecheck
bun run build
~~~

Expected: all checks pass. If the deployment defaults test still expects 20/80 while Compose and .env.production.example use 40/120, update the assertion to 40/120 and rerun that test.

- [ ] **Step 2: Build and replace only the local application container**

Run from the repository root:

~~~powershell
$env:TRUSTED_PROXIES='127.0.0.1/32,172.16.0.0/12'
docker compose build new-api
docker compose up -d --no-deps new-api
~~~

Expected: PostgreSQL and Redis are not recreated. new-api-dev starts and logs show successful migrations.

- [ ] **Step 3: Compare post-migration data and schema**

Run:

~~~powershell
docker exec new-api-dev-pg sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -AtF "|" -c "SELECT relname,n_live_tup FROM pg_stat_user_tables ORDER BY relname"' | Set-Content -Encoding utf8 .codex-tmp/rc33-phase1-backup/post-counts.tsv
docker exec new-api-dev-pg sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --schema-only --no-owner --no-privileges' | Set-Content -Encoding utf8 .codex-tmp/rc33-phase1-backup/post-schema.sql
~~~

Expected: critical business table row counts are unchanged. Schema differences are limited to expected indexes or GORM metadata changes.

- [ ] **Step 4: Exercise browser and API workflows**

Verify these local routes:

~~~text
/sign-in
/dashboard/overview
/wallet
/user-redemptions
/pricing
/playground
/system-settings/models/catalog-visibility
/usage-logs/common
~~~

Create and query a disposable user redemption code, list API Keys without changing stored key material, load model catalog visibility, upload text and image attachments in Playground, and execute one non-streaming plus one streaming request through the designated local test channel.

Expected: no logout, missing redemption, catalog load error, attachment error, or billing settlement 500.

- [ ] **Step 5: Record evidence and commit logical changes separately**

Record commands, exit status, table-count comparison, restore result, Docker image ID, and UI/API outcomes in verification.md. Stage only files changed by this plan. Leave the user's existing controller/user.go, main.go, model/token.go, and related test changes out of unrelated commits.
