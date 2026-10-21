package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readDeploymentFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func TestProductionDockerfileRunsAsNonRootWithPatchedToolchain(t *testing.T) {
	dockerfile := readDeploymentFile(t, "Dockerfile")

	assert.Regexp(t, `ARG GO_IMAGE=golang:1\.\d+\.\d+-alpine`, dockerfile)
	assert.Regexp(t, `ARG GO_IMAGE=golang:1\.26\.6-alpine@sha256:[0-9a-f]{64}`, dockerfile)
	assert.Contains(t, dockerfile, "ARG BUN_IMAGE=oven/bun:1@sha256:")
	assert.Contains(t, dockerfile, "ARG RUNTIME_IMAGE=debian:bookworm-slim@sha256:")
	assert.Equal(t, 2, strings.Count(dockerfile, "FROM ${BUN_IMAGE}"))
	assert.Contains(t, dockerfile, "FROM ${GO_IMAGE} AS builder2")
	assert.Contains(t, dockerfile, "FROM ${RUNTIME_IMAGE}")
	assert.Contains(t, dockerfile, "USER 10001:10001")
	assert.Contains(t, dockerfile, "install -d -o newapi -g newapi")
	assert.NotContains(t, dockerfile, "libasan8")
}

func TestProductionDockerfileAppliesRuntimeSecurityUpdates(t *testing.T) {
	dockerfile := readDeploymentFile(t, "Dockerfile")

	assert.Contains(t, dockerfile, "apt-get upgrade -y")
}

func TestProductionDockerfileUsesCompleteWorkspaceForBothFrontendBuilds(t *testing.T) {
	dockerfile := readDeploymentFile(t, "Dockerfile")

	assert.Equal(t, 2, strings.Count(dockerfile, "RUN bun install --frozen-lockfile"))
	assert.Equal(t, 2, strings.Count(dockerfile, "COPY ./web/shared ./shared"))
	assert.NotContains(t, dockerfile, "bun install --filter ./classic")
}

func TestProductionDockerfileAllowsGoProxyOverride(t *testing.T) {
	dockerfile := readDeploymentFile(t, "Dockerfile")

	assert.Contains(t, dockerfile, "ARG GOPROXY=https://proxy.golang.org,direct")
	assert.Contains(t, dockerfile, "ENV GOPROXY=${GOPROXY}")
	assert.Less(t, strings.Index(dockerfile, "ARG GOPROXY="), strings.Index(dockerfile, "RUN go mod download"))
}

func TestProductionDockerfileStripsLineEndingsFromEmbeddedVersion(t *testing.T) {
	dockerfile := readDeploymentFile(t, "Dockerfile")

	assert.Equal(t, 3, strings.Count(dockerfile, `tr -d '\r\n'`))
	assert.NotContains(t, dockerfile, "$(cat /build/VERSION)")
	assert.NotContains(t, dockerfile, "$(cat VERSION)")
}

func TestLocalBuildFilesStripLineEndingsFromEmbeddedVersion(t *testing.T) {
	developmentDockerfile := readDeploymentFile(t, "Dockerfile.dev")
	makefile := readDeploymentFile(t, "makefile")

	assert.Contains(t, developmentDockerfile, `$(tr -d '\r\n' < VERSION)`)
	assert.Equal(t, 2, strings.Count(makefile, `$$(tr -d '\r\n' < ../../VERSION)`))
	assert.NotContains(t, developmentDockerfile, "$(cat VERSION)")
	assert.NotContains(t, makefile, "$(cat ../../VERSION)")
}

func TestProductionImageUsesScratchRuntimeAndStaticHealthcheck(t *testing.T) {
	dockerfile := readDeploymentFile(t, "Dockerfile")
	compose := readDeploymentFile(t, "docker-compose.yml")

	assert.Contains(t, dockerfile, "FROM ${RUNTIME_IMAGE} AS runtime-files")
	assert.Contains(t, dockerfile, "RUN go build -trimpath -buildvcs=false -ldflags \"-s -w\" -o healthcheck ./cmd/healthcheck")
	assert.Contains(t, dockerfile, "FROM scratch")
	assert.Contains(t, dockerfile, "COPY --from=builder2 /build/healthcheck /healthcheck")
	assert.Contains(t, compose, `test: ["CMD", "/healthcheck"]`)
	assert.NotContains(t, compose, "wget")
}

func TestContainerHealthcheckValidatesBoundedStatusResponse(t *testing.T) {
	source := readDeploymentFile(t, "cmd/healthcheck/main.go")

	assert.Contains(t, source, "http.Client{Timeout:")
	assert.Contains(t, source, "io.LimitReader")
	assert.Contains(t, source, "response.StatusCode")
	assert.Contains(t, source, "status.Success")
}

func TestProductionComposeFailsClosedWithoutSecrets(t *testing.T) {
	compose := readDeploymentFile(t, "docker-compose.yml")

	assert.Contains(t, compose, "build:")
	assert.Contains(t, compose, "dockerfile: Dockerfile")
	assert.NotContains(t, compose, "calciumion/new-api:latest")
	assert.NotContains(t, compose, "123456")
	assert.Contains(t, compose, "${POSTGRES_PASSWORD:?")
	assert.Contains(t, compose, "${REDIS_PASSWORD:?")
	assert.Contains(t, compose, "${SESSION_SECRET:?")
	assert.Contains(t, compose, `DEPLOYMENT_MODE: "production"`)
	assert.Contains(t, compose, "SESSION_COOKIE_SECURE: \"true\"")
	assert.Contains(t, compose, "TLS_INSECURE_SKIP_VERIFY: \"false\"")
	assert.Contains(t, compose, "no-new-privileges:true")
	assert.Contains(t, compose, "cap_drop:")
	assert.Contains(t, compose, "- ALL")
	assert.True(t, strings.Contains(compose, "read_only: true"), "production container root filesystem must be read-only")
}

func TestProductionComposeBoundsPoolsAndAllowsGracefulShutdown(t *testing.T) {
	compose := readDeploymentFile(t, "docker-compose.yml")

	assert.Contains(t, compose, `SQL_MAX_IDLE_CONNS: "${SQL_MAX_IDLE_CONNS:-40}"`)
	assert.Contains(t, compose, `SQL_MAX_OPEN_CONNS: "${SQL_MAX_OPEN_CONNS:-120}"`)
	assert.Contains(t, compose, `REDIS_POOL_SIZE: "${REDIS_POOL_SIZE:-20}"`)
	assert.Contains(t, compose, `RELAY_RESPONSE_HEADER_TIMEOUT: "${RELAY_RESPONSE_HEADER_TIMEOUT:-300}"`)
	assert.Contains(t, compose, `SHUTDOWN_TIMEOUT_SECONDS: "${SHUTDOWN_TIMEOUT_SECONDS:-120}"`)
	assert.Contains(t, compose, "stop_grace_period: 2m")
	assert.Contains(t, compose, "pids_limit: 512")
}

func TestDevelopmentComposePinsTrustedProxyGateway(t *testing.T) {
	compose := readDeploymentFile(t, "docker-compose.dev.yml")

	assert.Contains(t, compose, "TRUSTED_PROXIES=172.30.250.1")
	assert.Contains(t, compose, "subnet: 172.30.250.0/24")
	assert.Contains(t, compose, "gateway: 172.30.250.1")
}

func TestProductionComposePinsInfrastructureImages(t *testing.T) {
	compose := readDeploymentFile(t, "docker-compose.yml")

	assert.Regexp(t, `image: postgres:16-alpine@sha256:[0-9a-f]{64}`, compose)
	assert.Regexp(t, `image: redis:7-alpine@sha256:[0-9a-f]{64}`, compose)
}

func TestProductionComposePassesCryptoLimitsAndAuditWorkerSettings(t *testing.T) {
	compose := readDeploymentFile(t, "docker-compose.yml")

	assert.Contains(t, compose, `${CRYPTO_SECRET:?set CRYPTO_SECRET}`)
	for _, name := range []string{
		"MAX_REQUEST_BODY_MB",
		"UPSTREAM_RESPONSE_BODY_LIMIT_MB",
		"MAX_FILE_DOWNLOAD_MB",
		"STREAM_SCANNER_MAX_BUFFER_MB",
		"ANONYMOUS_REQUEST_BODY_LIMIT_KB",
		"API_REQUEST_BODY_LIMIT_KB",
		"AUDIT_OUTBOX_INTERVAL_SECONDS",
		"AUDIT_OUTBOX_BATCH_SIZE",
		"AUDIT_OUTBOX_RETENTION_DAYS",
		"IP_SECURITY_PROJECTION_RETRY_SECONDS",
		"IP_SECURITY_PROJECTION_BATCH_SIZE",
	} {
		assert.Contains(t, compose, name+":", "%s must be passed into the production container", name)
	}
}

func TestDockerBuildContextExcludesSecretsDataAndAuditArtifacts(t *testing.T) {
	dockerignore := readDeploymentFile(t, ".dockerignore")

	for _, pattern := range []string{
		".env*",
		"*.db",
		"data/",
		"logs/",
		".local-run/",
		".verification-output/",
		"output/",
		"release/",
		".build-cache/",
		".codex-tmp/",
		".task9-acceptance/",
		".tmp*/",
		"validation-shots/",
	} {
		assert.Contains(t, dockerignore, pattern)
	}
}

func TestProductionEnvironmentExampleUsesCurrentImageVersion(t *testing.T) {
	environment := readDeploymentFile(t, ".env.production.example")

	assert.Contains(t, environment, "CLOVE_API_IMAGE_TAG=v1.0.5-beta")
	assert.NotContains(t, environment, "v1.0.0-rc.21.clove-enhanced")
	assert.Contains(t, environment, "CRYPTO_SECRET=replace_with_a_distinct_32_byte_random_value")
	assert.Contains(t, environment, "SQL_MAX_OPEN_CONNS=120")
	assert.Contains(t, environment, "RELAY_RESPONSE_HEADER_TIMEOUT=300")
	assert.Contains(t, environment, "SHUTDOWN_TIMEOUT_SECONDS=120")
}

func TestDisplayVersionDoesNotReplaceMachineVersionSurfaces(t *testing.T) {
	constants := readDeploymentFile(t, "common/constants.go")
	statusController := readDeploymentFile(t, "controller/misc.go")
	setupController := readDeploymentFile(t, "controller/setup.go")
	modelSetup := readDeploymentFile(t, "model/main.go")
	versionFile := strings.TrimSpace(readDeploymentFile(t, "VERSION"))
	compose := readDeploymentFile(t, "docker-compose.yml")

	assert.Contains(t, constants, `const DisplayVersion = "Clove API v1.0.5-beta"`)
	assert.Contains(t, statusController, `"version":                     common.DisplayVersion`)
	assert.Contains(t, setupController, "Version:       common.Version")
	assert.Contains(t, modelSetup, "Version:       common.Version")
	assert.Equal(t, "v1.0.5-beta", versionFile)
	assert.Contains(t, compose, "v1.0.5-beta")
	assert.NotContains(t, versionFile, "Clove API")
}

func TestSystemdUnitUsesDedicatedIdentityAndSandbox(t *testing.T) {
	unit := readDeploymentFile(t, "new-api.service")

	assert.Contains(t, unit, "User=new-api")
	assert.Contains(t, unit, "EnvironmentFile=/etc/new-api/new-api.env")
	assert.Contains(t, unit, "ExecStart=/opt/new-api/new-api --log-dir /var/log/new-api")
	assert.Contains(t, unit, "NoNewPrivileges=true")
	assert.Contains(t, unit, "ProtectSystem=strict")
	assert.Contains(t, unit, "ReadWritePaths=/var/lib/new-api /var/log/new-api")
	assert.Contains(t, unit, "TimeoutStopSec=120")
}

func TestProductionOperationsGuideCoversRecoveryAndMonitoring(t *testing.T) {
	guide := readDeploymentFile(t, "docs/production-deployment.md")

	for _, section := range []string{
		"## 发布前检查",
		"## 备份与恢复",
		"## 发布与回滚",
		"## 监控与告警",
	} {
		assert.Contains(t, guide, section)
	}
	assert.Contains(t, guide, "/api/status")
	assert.Contains(t, guide, "audit_outbox")
	assert.Contains(t, guide, "ip_security_projection")
}

func TestReleaseWorkflowsInjectVersionUsingCurrentModulePath(t *testing.T) {
	const versionSymbol = "github.com/QuantumNous/new-api/common.Version"
	const obsoleteVersionSymbol = "'new-api/common.Version="

	for _, path := range []string{
		".github/workflows/release.yml",
		".github/workflows/electron-build.yml",
	} {
		workflow := readDeploymentFile(t, path)

		assert.Contains(t, workflow, versionSymbol, "%s must inject the build version", path)
		assert.NotContains(t, workflow, obsoleteVersionSymbol, "%s uses the obsolete linker symbol", path)
	}
}

func TestReleaseWorkflowsPinToolchainsAndUseWorkspaceInstall(t *testing.T) {
	for _, path := range []string{
		".github/workflows/release.yml",
		".github/workflows/electron-build.yml",
	} {
		workflow := readDeploymentFile(t, path)

		assert.Contains(t, workflow, "bun-version: '1.3.14'", "%s must pin Bun", path)
		assert.Contains(t, workflow, "go-version: '1.26.6'", "%s must pin Go", path)
		assert.NotContains(t, workflow, "bun-version: latest", "%s must not use a moving Bun release", path)
		assert.NotContains(t, workflow, "bun install --filter ./classic", "%s must use the complete workspace install", path)
	}

	electronWorkflow := readDeploymentFile(t, ".github/workflows/electron-build.yml")
	assert.Contains(t, electronWorkflow, "bun install --frozen-lockfile")
	assert.Contains(t, electronWorkflow, "cd default")
	assert.Contains(t, electronWorkflow, "cd ../classic")
	assert.Contains(t, electronWorkflow, "npm ci")
}

func TestElectronDocumentationHasExecutableBuildInstructions(t *testing.T) {
	documentation := readDeploymentFile(t, "electron/README.md")

	assert.NotContains(t, documentation, "TODO")
	assert.Contains(t, documentation, "bun install --frozen-lockfile")
	assert.Contains(t, documentation, "cd default")
	assert.Contains(t, documentation, "cd ../classic")
	assert.Contains(t, documentation, "go build")
	assert.Contains(t, documentation, "npm ci")
}
