package secrets

import "errors"

// ErrVaultNotConfigured indicates VaultProvider is not wired in this build.
var ErrVaultNotConfigured = errors.New("vault provider not configured")

// UnconfiguredVault is a stub VaultProvider; use EnvProvider in production today.
type UnconfiguredVault struct{}

func (UnconfiguredVault) GetSecret(string) string { return "" }

func (UnconfiguredVault) Ready() bool { return false }
