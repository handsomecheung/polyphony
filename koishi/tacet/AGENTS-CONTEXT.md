# AI Agent Context for Tacet

`tacet` is a zero-knowledge CLI secret manager and environment injector compatible with Bitwarden and self-hosted Vaultwarden servers. It consists of a stateless CLI (`tacet`) and a background daemon (`tacetd`) communicating over a secure Unix Domain Socket (UDS).

This document outlines the architecture, cryptographic pipelines, security invariants, API specifications, and development rules for AI Agents operating on `koishi/tacet`.

---

## 1. Core Rules & Policies for Agents

1. **Security & Zeroization Invariants:**
   - **Never Persist Plaintext Secrets:** Sensitive values (passwords, tokens, TOTP seeds, decrypted notes/fields) must never be written to persistent storage or unredacted log files.
   - **Zeroize Keys on Drop:** All cryptographic keys (`SymmetricKey`, `StretchedMasterKey`, raw key slices) must implement `zeroize::Zeroize` or be wrapped in `zeroize::Zeroizing<T>`.
   - **Master Password Handling:** Master passwords must be wiped immediately after deriving KDF output.
   - **Strict File Permissions:** Any file containing authentication tokens or cached ciphers (`session.json`, `vault.cache`) must be created with Unix mode `0600`; enclosing directories must be `0700`.
   - **No TCP Listeners:** The daemon must never bind to network TCP sockets (`127.0.0.1`, `0.0.0.0`). It must exclusively communicate via Unix Domain Sockets (`tacetd.sock`).
   - **Peer Validation:** The daemon must inspect connecting socket peers (`SO_PEERCRED`) and reject any process whose UID does not match the daemon process UID with HTTP `403 Forbidden`.

2. **Code Integrity & Conventions:**
   - When comments are needed, write them in **English**.
   - Maintain idiomatic Rust code conforming to Rust 2021 edition.
   - Follow strict error handling with typed error enums (`thiserror`) and avoid panics (`unwrap()` / `expect()`) in production code paths.
   - Changes must pass `cargo test`, `cargo clippy --all-targets -- -D warnings`, and `cargo fmt --check`.

3. **Git State Rule:**
   - Unless explicitly instructed otherwise by the user, **do not change the Git state** (do not stage changes, create commits, or push).

---

## 2. Project Architecture & Codebase Structure

```text
koishi/tacet/
├── Cargo.toml
├── scripts/
│   └── build.sh                # Release build and ~/bin/ installation script
├── src/
│   ├── lib.rs
│   ├── bin/
│   │   ├── tacet.rs            # CLI binary entrypoint
│   │   └── tacetd.rs           # Daemon binary entrypoint
│   ├── crypto/                 # Bitwarden-compatible cryptographic primitives
│   │   ├── mod.rs
│   │   ├── cipher_string.rs    # Type 2 (AES-CBC+HMAC) & Type 4 (RSA-OAEP) parsing/decryption
│   │   ├── hkdf.rs             # HKDF-Expand for encryption & MAC keys
│   │   ├── kdf.rs              # PBKDF2-SHA256 & Argon2id master key derivation
│   │   ├── key.rs              # Zeroizing SymmetricKey custody container
│   │   └── password_hash.rs    # MasterPasswordHash derivation for upstream OAuth
│   ├── upstream/               # Bitwarden / Vaultwarden API client
│   │   ├── mod.rs
│   │   ├── client.rs           # reqwest-based API client (prelogin, token, sync)
│   │   ├── endpoints.rs        # URL discovery for official & self-hosted servers
│   │   └── models.rs           # Serde models for sync, tokens, and ciphers
│   ├── storage/                # Local encrypted storage & configuration
│   │   ├── mod.rs
│   │   ├── paths.rs            # XDG base directories (~/.config/tacet, ~/.cache/tacet, $XDG_RUNTIME_DIR)
│   │   ├── session.rs          # Session metadata persistence (tokens, server URL, key hashes)
│   │   └── vault_cache.rs      # Encrypted vault cache persistence (vault.cache)
│   ├── uri/                    # tacet:// URI parser & AST
│   │   ├── mod.rs
│   │   ├── parser.rs           # URI syntax parser (scope, item, field, query modifiers)
│   │   └── types.rs            # Scope, FieldTarget, and QueryModifiers types
│   ├── resolver/               # Secret resolution engine
│   │   ├── mod.rs
│   │   ├── engine.rs           # Vault traversal, item matching, and field extraction
│   │   ├── json_path.rs        # Nested JSON dot-path evaluation
│   │   └── totp.rs             # Live RFC 6238 TOTP calculation (totp-rs)
│   ├── daemon/                 # HTTP-over-UDS server (axum)
│   │   ├── mod.rs
│   │   ├── app.rs              # axum router setup
│   │   ├── routes.rs           # REST endpoints (/v1/status, login, unlock, lock, logout, sync, resolve, list)
│   │   ├── server.rs           # UnixListener bind, socket permission enforcement & peer UID check
│   │   └── state.rs            # In-memory custody state and idle auto-lock background timer
│   └── cli/                    # CLI execution & process management
│       ├── mod.rs
│       ├── client.rs           # HTTP client over Unix Domain Socket
│       ├── commands/           # Subcommands (login, unlock, lock, logout, status, sync, list, read, inject, run)
│       ├── masker.rs           # Real-time stdout/stderr secret stream redaction
│       ├── run.rs              # Process spawn, env injection, and streaming output handler
│       └── autospawn.rs        # Automatic background daemon spawning wrapper
└── tests/                      # Integration test suites
    ├── cli_tests.rs
    ├── crypto_tests.rs
    ├── daemon_tests.rs
    ├── resolver_tests.rs
    ├── storage_tests.rs
    ├── upstream_tests.rs
    └── uri_tests.rs
```

---

## 3. Cryptographic Pipeline Specifications

All cryptographic operations strictly adhere to the Bitwarden Cryptographic Specification:

```
[Master Password] + [Email (lowercase, trimmed)]
                       │
                       ▼
       ┌───────────────────────────────┐
       │ KDF: PBKDF2-SHA256 / Argon2id │
       └───────────────┬───────────────┘
                       │
                       ▼
               [Master Key (32B)]
               /               \
              /                 \
             ▼                   ▼
┌─────────────────────────┐   ┌─────────────────────────┐
│ PBKDF2-SHA256 (1 iter)  │   │ HKDF-Expand (SHA-256)   │
│ salt = Master Password  │   │ info = b"enc" / b"mac"  │
└────────────┬────────────┘   └────────────┬────────────┘
             │                             │
             ▼                             ▼
   [MasterPasswordHash]           [StretchedMasterKey (64B)]
             │                             │
             ▼                             ▼
   POST /identity/token          Decrypt Protected SymmetricKey
             │                             │
             ▼                             ▼
       [OAuth Token]                 [SymmetricKey (64B)]
                                     ├── EncKey (32B)
                                     └── MacKey (32B)
```

1. **Prelogin (`POST /identity/accounts/prelogin`):**
   - Retrieves KDF type: `0` (PBKDF2-SHA256) or `1` (Argon2id).
   - Parameters: PBKDF2 iterations (default: 600,000) or Argon2id iterations, memory (MB), and parallelism.
2. **Master Key Derivation (32 Bytes):**
   - Derived using `master_password` with `email.trim().to_lowercase()` as the salt.
3. **Master Password Hash & Upstream Authentication:**
   - `Base64(PBKDF2-SHA256(password = MasterKey, salt = master_password, iterations = 1, dkLen = 32))`
   - Authenticates against `POST {server}/identity/connect/token` via `grant_type=password`.
   - Submits client device parameters required by Vaultwarden/Bitwarden: `deviceType` (`25` on Linux), `deviceIdentifier` (persistent UUID from `~/.config/tacet/device_id`), and `deviceName` (`tacet-cli`).
4. **Stretched Master Key (64 Bytes):**
   - HKDF-Expand with SHA-256: `EncKey = HKDF(MasterKey, info = b"enc", 32)` and `MacKey = HKDF(MasterKey, info = b"mac", 32)`.
5. **Decryption of User Symmetric Key:**
   - The user's encrypted `Key` is returned during login as a `Type 2` CipherString.
   - Decrypted using `StretchedMasterKey` to yield the final 64-byte `SymmetricKey`.
   - Master password and intermediate master keys are immediately zeroized.
6. **CipherString Formats:**
   - **Type 2 (`Aes256_HmacSha256_B64`):** Format `2.<iv_b64>|<ciphertext_b64>|<mac_b64>`. Decryption verifies constant-time HMAC-SHA256 over `iv + ciphertext` with `MacKey` before AES-256-CBC decryption with `EncKey`.
   - **Type 4 (`Rsa2048_OaepSha1_B64`):** Format `4.<ciphertext_b64>`. Decryption uses RSA-2048 private key with OAEP padding and SHA-1 digest (used for organization symmetric keys).

---

## 4. `tacetd` HTTP-over-UDS API Contract

### Transport & Security
- **Socket Path:** `$XDG_RUNTIME_DIR/tacet/tacetd.sock` (fallback: `/tmp/tacet-$UID/tacetd.sock`).
- **Permissions:** Directory `0700`, socket file `0600`.
- **Peer Check:** Peer UID verified on connection; mismatches return `403 Forbidden`.
- **Auto-Lock:** Background task locks in-memory keys after `idle_ttl` (default: 30 minutes).

### REST Endpoints

| Method & Path | Request Body | Success Response (`200 OK`) | Error Codes | Notes |
|---|---|---|---|---|
| `GET /v1/status` | None | `{"running": true, "logged_in": bool, "unlocked": bool, "server_url": string, "item_count": u32}` | — | Health and state check |
| `POST /v1/login` | `{"server_url": string, "email": string, "master_password": string, "two_factor_token": string?}` | `{"unlocked": true, "item_count": u32}` | 400, 401, 502 | Upstream auth, initial sync, unlock |
| `POST /v1/unlock` | `{"master_password": string}` | `{"unlocked": true}` | 400, 401 | Restores SymmetricKey into memory |
| `POST /v1/lock` | None | `{"locked": true}` | — | Zeroizes SymmetricKey immediately |
| `POST /v1/logout` | None | `{"logged_out": true}` | — | Zeroizes keys and purges local storage |
| `POST /v1/sync` | None | `{"synced": true, "item_count": u32}` | 401, 502 | Refreshes token & updates vault cache |
| `POST /v1/resolve` | `{"uris": [string]}` | `{"secrets": {string: string}}` | 400, 401, 404, 409 | Batch secret resolution |
| `GET /v1/list` | None (Query: `?scope=..&search=..`) | `{"items": [{"id": "..", "name": "..", "scope": "..", "folder": "..", "cipher_type": "..", "uri_prefix": ".."}], "total": usize}` | 401, 500 | Lists item summaries with decrypted names |

---

## 5. CLI Execution & Safety Guarantees

1. **Automatic Daemon Management (`ensure_daemon_running`):**
   - The CLI tests connectivity to the UDS socket before executing commands.
   - If unreachable, it forks/spawns `tacetd` detached and polls `/v1/status` until ready (timeout: 5 seconds).
2. **Fail-Fast Policy:**
   - In `tacet run` and `tacet inject`, if any URI fails to resolve (e.g. locked vault, missing item, ambiguous match), the command aborts immediately with a non-zero exit code before child process execution.
3. **Real-Time Stream Masking (`tacet run`):**
   - Secrets of length $\ge 4$ characters are registered into a thread-safe stream filter.
   - `stdout` and `stderr` streams of the child process are monitored line-by-line / chunk-by-chunk and matching secret tokens are redacted to `[TACET_MASKED]`.

---

## 6. Development & Verification Workflows

When implementing or modifying features:

```bash
# 1. Run all unit and integration tests
cargo test

# 2. Check clippy lints with warnings treated as errors
cargo clippy --all-targets -- -D warnings

# 3. Check code formatting
cargo fmt --check

# 4. Verify binary builds
cargo build --bin tacet --bin tacetd
```
