package common

import "crypto/tls"

// InsecureTLSConfig is retained for source compatibility with official RC20
// call sites that are kept byte-for-byte identical.
var InsecureTLSConfig = &tls.Config{ // #nosec G402 -- admin-controlled relay compatibility option.
	MinVersion:         tls.VersionTLS12,
	InsecureSkipVerify: true, // #nosec G402 -- production startup rejects this admin compatibility mode.
}

func configureTLSInsecureSkipVerify(enabled bool) {
	TLSInsecureSkipVerify = enabled
}

func RelayTLSClientConfig() *tls.Config {
	if !TLSInsecureSkipVerify {
		return nil
	}
	return &tls.Config{ // #nosec G402 -- admin-controlled relay compatibility option.
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // #nosec G402 -- production startup rejects this admin compatibility mode.
	}
}
