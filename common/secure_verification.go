package common

// SecureVerificationSessionKey is the session key recorded when a user passes
// step-up verification (2FA or passkey). Centralized here so controller and
// middleware share a single definition instead of duplicating the value.
const SecureVerificationSessionKey = "secure_verified_at"

// SecureVerificationTimeout is the validity window (seconds) for a passed
// step-up verification. After this many seconds the user must re-verify.
const SecureVerificationTimeout = 300 // 5 minutes
