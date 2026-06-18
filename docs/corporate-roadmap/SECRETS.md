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

Then rotate `JWT_SECRET` to a new value in a maintenance window (see slice `15-secret-rotation-doc-env` for the full runbook). Existing JWTs signed with the old secret stop working after rotation.

### What does *not* change

- Provider API keys, MCP credentials, and other ciphertext in the database remain tied to `DB_ENCRYPTION_KEY` only.
- Rotating `JWT_SECRET` invalidates active login sessions but does **not** re-encrypt database fields.

## Validation

On startup the server calls `auth.ValidateStartupSecrets()` and exits with a clear error if:

- `DB_ENCRYPTION_KEY` is not exactly 32 characters
- `JWT_SECRET` is empty or shorter than 16 characters
