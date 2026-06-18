# Secrets configuration

Aigent uses two independent environment secrets. Do not reuse the same value for both unless you are performing a one-time migration (see below).

## Variables

| Variable | Purpose | Requirements |
|----------|---------|--------------|
| `DB_ENCRYPTION_KEY` | AES-256-GCM encryption for provider API keys, MCP env maps, HandsAI tokens, and other fields stored in PostgreSQL | Exactly **32 characters** |
| `JWT_SECRET` | HMAC-SHA256 signing key for login session JWTs | **Required**, at least **16 characters** (use 32+ random bytes in production) |

## Generate values

```bash
# 32-char AES key (example)
openssl rand -hex 16

# JWT secret (example, 32+ chars)
openssl rand -base64 32
```

Add both to `.env`:

```env
DB_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
JWT_SECRET=your-long-random-jwt-signing-secret
```

## Migration from single-key setups

Older Aigent builds used `DB_ENCRYPTION_KEY` for **both** database encryption and JWT signing.

1. **Keep** your existing `DB_ENCRYPTION_KEY` unchanged so encrypted database fields continue to decrypt.
2. **Add** a new `JWT_SECRET` before starting the upgraded server.
3. **Restart** the application. Startup fails if either variable is missing or invalid.

### Minimal downtime option

To avoid forcing all users to log in immediately, you may temporarily set:

```env
JWT_SECRET=<same value as DB_ENCRYPTION_KEY>
```

Then rotate `JWT_SECRET` to a new value in a maintenance window (see [Rotation runbook](#rotation-runbook) below). Existing JWTs signed with the old secret stop working after rotation.

### What does *not* change

- Provider API keys, MCP credentials, and other ciphertext in the database remain tied to `DB_ENCRYPTION_KEY` only.
- Rotating `JWT_SECRET` invalidates active login sessions but does **not** re-encrypt database fields.

## Validation

On startup the server calls `auth.ValidateStartupSecrets()` and exits with a clear error if:

- `DB_ENCRYPTION_KEY` is not exactly 32 characters
- `JWT_SECRET` is empty or shorter than 16 characters

## Secret adapter (env / future vault)

Application code reads secrets through `internal/secrets`:

```go
key, err := secrets.RequireDBEncryptionKey()
jwt := secrets.JWTSecret()
raw := secrets.GetSecret(secrets.KeyJWTSecret)
```

- **Default:** `EnvProvider` reads `os.Getenv` (works with Docker Compose `.env`, Kubernetes `secretKeyRef`, Dokploy env injection).
- **Future:** implement `VaultProvider` (stub in `internal/secrets/vault_stub.go`) and call `secrets.SetProvider` at startup.

### Kubernetes example

```yaml
env:
  - name: DB_ENCRYPTION_KEY
    valueFrom:
      secretKeyRef:
        name: aigent-secrets
        key: db-encryption-key
  - name: JWT_SECRET
    valueFrom:
      secretKeyRef:
        name: aigent-secrets
        key: jwt-secret
```

No code changes are required when moving from `.env` to mounted secrets — only how the variables are injected.

## Rotation runbook

### JWT_SECRET rotation (low risk)

**Impact:** Active login sessions invalidated; users must sign in again. No database ciphertext is affected.

1. Generate a new secret: `openssl rand -base64 32`
2. Schedule a short maintenance window (optional — rotation is instant on restart).
3. Update `JWT_SECRET` in your secret store / `.env`.
4. Restart the Aigent server (`docker compose up -d` or rolling pod restart).
5. Verify login works with the admin account and that API calls return `401` for old tokens.
6. Communicate to operators that they need to re-authenticate.

**Rollback:** Restore the previous `JWT_SECRET` and restart. Old tokens work again until they expire (24h default).

### DB_ENCRYPTION_KEY rotation (high risk)

**Impact:** All AES-encrypted fields (LLM provider API keys, MCP env maps, HandsAI token in DB) were encrypted with the current key. Changing the key **without re-encrypting** makes existing ciphertext unreadable.

Aigent does **not** yet automate dual-key or online re-encryption. Plan carefully:

#### Option A — Planned migration (recommended)

1. **Backup** PostgreSQL (`pg_dump`) before any change.
2. Export secrets you can rotate out-of-band:
   - Re-enter provider API keys via the UI after rotation, or
   - Decrypt offline with a one-off script using the **old** key, then re-save via API after switching keys.
3. Generate new 32-char key: `openssl rand -hex 16`
4. Stop the server.
5. Replace `DB_ENCRYPTION_KEY` in secrets / `.env`.
6. Start the server — startup validation passes with the new key.
7. **Re-configure** each provider, MCP server env, and HandsAI token (connection test re-encrypts with the new key).
8. Run smoke tests: provider test, MCP dry-run, chat with tools.

#### Option B — Emergency rollback

If the new key was applied by mistake and ciphertext is already broken:

1. Stop the server.
2. Restore `DB_ENCRYPTION_KEY` to the **previous** value.
3. Restore DB from backup if partial writes occurred.
4. Restart and verify decrypt paths (provider test, MCP list).

### Rotation checklist

| Step | JWT_SECRET | DB_ENCRYPTION_KEY |
|------|------------|-------------------|
| DB backup | Optional | **Required** |
| Downtime | ~seconds (restart) | Minutes (re-enter secrets) |
| User re-login | Yes | No |
| Re-enter provider keys | No | Yes |
| Automated in-app | Yes (env swap) | No (manual re-config) |

### Post-rotation verification

```bash
# Health: server starts without FATAL secret errors
go run cmd/server/main.go

# Encrypt/decrypt regression (CI)
go test ./internal/secrets/... ./internal/utils/...
```

- Log in via UI → new JWT issued.
- Providers tab → **Test connection** on each provider.
- MCP stdio/stream → **Test** saved server.
- Confirm chat can call tools that need stored credentials.
