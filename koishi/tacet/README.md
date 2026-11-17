# Tacet

A fast, zero-knowledge CLI secret manager and environment injector compatible with Bitwarden and self-hosted Vaultwarden servers.

## Architecture

Tacet consists of two primary binaries:
- `tacet`: A stateless CLI client for reading secrets, injecting environment variables, and masking output.
- `tacetd`: An `axum`-powered background daemon listening on a Unix Domain Socket (`tacetd.sock`), managing authenticated sessions, encrypted vault cache, and in-memory key custody.

## Current Status (MVP Phase 1 & 2)

### Phase 1: Core Crypto, Upstream Client & Storage
- **Crypto (`crypto`):**
  - Master key derivation with PBKDF2-SHA256 and Argon2id.
  - Master password hash derivation for Bitwarden upstream authentication.
  - HKDF stretching into 64-byte symmetric encryption and MAC verification keys.
  - Constant-time HMAC-SHA256 verification and AES-256-CBC decryption (`Type 2` CipherStrings).
  - RSA-2048 OAEP-SHA1 decryption (`Type 4` CipherStrings).
  - In-memory key zeroization on drop.
- **Upstream (`upstream`):**
  - Endpoint discovery for official Bitwarden cloud and custom self-hosted Vaultwarden URLs.
  - Prelogin, OAuth 2.0 token grant, token refresh, and vault sync (`/api/sync`).
  - Two-factor authentication (2FA) challenge detection.
- **Storage (`storage`):**
  - XDG-compliant storage paths (`~/.config/tacet` and `~/.cache/tacet`).
  - Session metadata and encrypted vault cache persistence with strict Unix permissions (`0700` dirs, `0600` files).

### Phase 2: URI Parser & Vault Resolver
- **URI Parser (`uri`):**
  - Parses `tacet://<scope>/<item>/<field>[?<modifiers>]` syntax.
  - Supports scopes: `personal` (or `_`), `personal/<folder>`, `org/<org_name>/<collection>`, `*`.
  - Item matching by exact 36-char UUID or case-insensitive decrypted name, supporting `%2F` literal slash escaping.
  - Field target mapping: built-in aliases (`username`/`user`, `password`/`pass`, `totp`/`otp`, `totp_secret`, `uri`/`url`/`uri.<n>`, `notes`/`note`), custom fields (`fields/<X>`), and smart `<X>` fallback.
  - Query modifiers: `encoding=base64|hex`, `json=<dot.path>`, `trim=true|false`.
- **Vault Resolver (`resolver`):**
  - Scope and folder/collection resolution with on-demand name decryption.
  - Automatic organization key unwrapping (`Type 4` RSA-OAEP-SHA1) using user RSA private key.
  - Fail-fast disambiguation error listing matching UUIDs and folders when multiple items match.
  - RFC 6238 TOTP code generation from `otpauth://` URIs or raw base32 seeds.
  - Nested JSON evaluation and modifier pipelines.

## Testing

Run all unit and integration tests:

```bash
cargo test
```
