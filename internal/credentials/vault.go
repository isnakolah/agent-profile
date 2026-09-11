// Package credentials stores API keys in the OS vault, never profile JSON.
package credentials

import (
	"crypto/sha256"
	"fmt"
)

func account(root, profile, provider string) string {
	return fmt.Sprintf("%x/%s/%s", sha256.Sum256([]byte(root)), profile, provider)
}

const service = "com.isnakolah.agent-profile"
