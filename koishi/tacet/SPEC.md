# Tacet (MVP) — Architecture & Implementation Specification

## 1. Project Overview
**Tacet** (Latin: *"it is silent"*) is a fast, zero-knowledge CLI secret manager and environment injector compatible with **Bitwarden** and self-hosted **Vaultwarden** servers. Inspired by the developer ergonomics of 1Password CLI (`op`), Tacet eliminates plaintext `.env` files on disk and provides `<5ms` secret resolution via a lightweight background daemon.

### Architectural Philosophy: Stateful Local Web Server (`tacetd`) + Stateless CLI (`tacet`)
* **`tacetd` (Stateful Background Daemon):** An `axum`-powered HTTP/1.1 REST server listening exclusively on a **Unix Domain Socket** (`tacetd.sock`). It acts as the single custody vault for Bitwarden OAuth tokens, local encrypted vault cache (`vault.cache`), and the in-memory 64-byte `SymmetricKey` (with idle auto-lock zeroization).
* **`tacet` (Stateless CLI Client):** A fast, short-lived binary that parses CLI flags/templates, sends standard HTTP requests over UDS to `tacetd`, injects resolved secrets into child process memory, and redacts `stdout`/`stderr` streams in real time.

---

## 2. Tech Stack (Rust)

* **Language:** Rust (Edition 2021/2024)
* **Binary Targets:** `tacet` (CLI client) and `tacetd` (background HTTP-over-UDS daemon, auto-spawned by `tacet` if not running)
* **Web Server & HTTP Client (over UDS):**
  * Server: `axum` (v0.7+) + `tokio::net::UnixListener` + `hyper` / `tower`
  * Client: `hyper` + `hyperlocal` (or custom `tower::Service` over `tokio::net::UnixStream`) for CLI-to-daemon requests, plus `reqwest` (rustls) for upstream Bitwarden/Vaultwarden API calls
* **CLI & Prompts:** `clap` (v4 derive), `rpassword` / `dialoguer`
* **Cryptography:**
  * `pbkdf2`, `argon2`, `sha2`, `sha1`, `hmac`, `hkdf`
  * `aes`, `cbc`, `rsa` (OAEP-SHA1 for organization keys)
  * `subtle` (constant-time MAC verification)
  * `zeroize` (`Zeroizing<T>` / `ZeroizeOnDrop` for in-memory key hygiene)
  * `totp-rs` (RFC 6238 dynamic TOTP code generation)
* **Serialization & Env:** `serde`, `serde_json`, `url`, `dotenvy`

---

## 3. User Lifecycle & Cryptographic Pipeline

### 3.1 User Lifecycle (UX Flow)
1. **First-time Setup (`tacet login`):**
   * Prompts for Server URL (defaults to `https://api.bitwarden.com`, or custom Vaultwarden URL), Email, Master Password, and 2FA code (if enabled).
   * Auto-starts `tacetd` if not running, sends credentials to `POST /v1/login` over UDS.
   * `tacetd` authenticates with upstream server, caches OAuth tokens + encrypted `vault.cache` to disk (`~/.config/tacet/` & `~/.cache/tacet/`), and keeps the derived 64-byte `SymmetricKey` in memory (**Unlocked** state).
2. **Subsequent Unlocks (`tacet unlock`):**
   * After idle TTL (default `30m`) or reboot, `tacetd` wipes the `SymmetricKey` from memory (**Locked** state).
   * Running `tacet unlock` prompts only for the **Master Password**, calls `POST /v1/unlock`, and restores the unlocked state.
3. **Daily Usage:**
   * While unlocked, `tacet read`, `tacet inject`, and `tacet run` call `POST /v1/resolve` over UDS and execute in `<5ms` with zero prompts.

### 3.2 Bitwarden Cryptographic Pipeline (Inside `tacetd`)
1. **Prelogin:** `POST {server}/identity/accounts/prelogin` $\rightarrow$ get `kdf` (`0` = PBKDF2-SHA256, `1` = Argon2id) and KDF parameters.
2. **MasterKey (32B):** Derived from `master_password` + `email.trim().to_lowercase()`.
3. **MasterPasswordHash:** `Base64(PBKDF2-SHA256(password = MasterKey, salt = master_password, iterations = 1, dkLen = 32))` $\rightarrow$ `POST {server}/identity/connect/token`.
4. **StretchedMasterKey (64B):** `HKDF-Expand(PRK = MasterKey, info = b"enc" / b"mac", L = 32)`.
5. **SymmetricKey (64B):** Decrypt login response's `Key` (`Type 2` CipherString) using `StretchedMasterKey`. Split into `EncKey[0..32]` and `MacKey[32..64]`. **Immediately zeroize** `master_password` and `MasterKey`.
6. **Vault Sync & Decryption:** `GET {server}/api/sync` $\rightarrow$ persist raw CipherStrings in `vault.cache`. Decrypt `Type 2` (`2.<iv>|<ct>|<mac>`, constant-time HMAC-SHA256 verification before AES-256-CBC decryption) on demand. Support `Type 4` (RSA-OAEP-SHA1) for organization collections.

---

## 4. `tacet://` URI Specification

### 4.1 Syntax
```text
tacet://<scope>/<item>/<field>[?<modifiers>]

```

### 4.2 `<scope>` Rules

| Scope                         | Resolution Target                                                    |
|-------------------------------|----------------------------------------------------------------------|
| `personal` or `_`             | Personal vault (`organizationId == null`), ignoring folder hierarchy |
| `personal/<folder>`           | Personal vault items inside specific `<folder>`                      |
| `org/<org_name>/<collection>` | Shared items inside an Organization's `<collection>`                 |
| `*`                           | Search all accessible vaults (fails if ambiguous)                    |

### 4.3 `<item>` Matching

* Case-insensitive match by Cipher `name`, or exact 36-char Cipher `UUID`.
* Supports spaces when quoted (e.g., `"tacet://personal/OpenAI API/api_key"`) and `%2F` for literal slashes.
* Fails fast with a disambiguation error listing matching UUIDs/folders if multiple items match.

### 4.4 `<field>` Mapping

| Bitwarden Source    | URI `<field>`         | Alias   | Behavior                                                     |
|---------------------|-----------------------|---------|--------------------------------------------------------------|
| `login.username`    | `username`            | `user`  | Plaintext username                                           |
| `login.password`    | `password`            | `pass`  | Plaintext password                                           |
| `login.totp`        | `totp`                | `otp`   | Live 6-digit RFC 6238 TOTP code                              |
| `login.totp` (raw)  | `totp_secret`         | -       | Raw TOTP seed / `otpauth://` URI                             |
| `login.uris[0].uri` | `uri` / `url`         | `uri.0` | Primary URL (`uri.<n>` for index `n`)                        |
| `notes`             | `notes`               | `note`  | Secure note body                                             |
| `fields[name == X]` | `fields/<X>` or `<X>` | -       | Checks built-in fields first; falls back to custom field `X` |

### 4.5 `[?<modifiers>]` (Optional)

* `encoding=base64` | `hex`: Encode multi-line secrets (e.g., PEM keys) into single-line strings.
* `json=<dot.path>`: Extract nested value if field/note contains JSON.
* `trim=true|false`: Strip trailing whitespace/newlines (default: `true`).

---

## 6. `tacetd` HTTP-over-UDS API Contract

### 6.1 Transport & Security

* **Socket Path:** `$XDG_RUNTIME_DIR/tacet/tacetd.sock` (fallback `/tmp/tacet-$UID/tacetd.sock`), directory mode `0700`, socket file mode `0600`.
* **No TCP Exposure:** Never bind `127.0.0.1` or `0.0.0.0` in MVP—eliminates browser CSRF/DNS-rebinding attacks and avoids local bearer token management.
* **Peer Verification:** Extract connecting peer UID via `UnixStream::peer_cred()` (`SO_PEERCRED` / `getpeereid`) and reject any connection where `peer_uid != daemon_uid` with `403 Forbidden`.
* **Auto-Lock Timer:** Background `tokio::time::interval` checks last activity timestamp; if elapsed `> idle_ttl` (default `30m`), drop/zeroize `SymmetricKey`.

### 6.2 REST Endpoints (JSON Payload)

| Method & Path      | Request Body                                                                                | Response Body (`200 OK`)                                                                         | Description                                                                |
|--------------------|---------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------|
| `GET /v1/status`   | —                                                                                           | `{"running": true, "logged_in": bool, "unlocked": bool, "server_url": "...", "item_count": u32}` | Daemon health & vault state                                                |
| `POST /v1/login`   | `{"server_url": "...", "email": "...", "master_password": "...", "two_factor_token": null}` | `{"unlocked": true, "item_count": u32}`                                                          | Upstream auth, initial sync, unlock daemon                                 |
| `POST /v1/unlock`  | `{"master_password": "..."}`                                                                | `{"unlocked": true}`                                                                             | Derive keys from cached profile & unlock                                   |
| `POST /v1/lock`    | —                                                                                           | `{"locked": true}`                                                                               | Immediately zeroize in-memory keys                                         |
| `POST /v1/logout`  | —                                                                                           | `{"logged_out": true}`                                                                           | Zeroize keys & wipe local tokens/cache                                     |
| `POST /v1/sync`    | —                                                                                           | `{"synced": true, "item_count": u32}`                                                            | Refresh `access_token` if needed & sync vault                              |
| `POST /v1/resolve` | `{"uris": ["tacet://...", ...]}`                                                            | `{"secrets": {"tacet://...": "value"}}`                                                          | Batch resolve URIs (returns `401` if locked, `404`/`409` on lookup errors) |

---

## 7. CLI Interface (`tacet`)

```text
# Session & Vault Management
tacet login [--server <url>] [--email <email>]   # Interactive login, initial sync & daemon start
tacet unlock                                     # Prompt Master Password to unlock tacetd
tacet lock                                       # Wipe keys in tacetd memory immediately
tacet logout                                     # Lock daemon & remove local tokens/cache
tacet status                                     # Show login state, server URL, lock state, item count
tacet sync                                       # Fetch latest encrypted vault from server

# Secret Consumption
tacet read <uri> [--no-newline]                  # Print single resolved secret to stdout
tacet inject [-i <tpl_file>] [-o <out_file>]     # Render {{ tacet://... }} placeholders (supports stdin/stdout)
tacet run [--env-file <file>] [--no-mask] -- <cmd> [args...]
                                                 # Inject resolved env vars into subprocess & mask output

```

### Key CLI Execution Guarantees

1. **Auto-Spawn Daemon:** Before sending HTTP requests to UDS, if the socket is unreachable, `tacet` spawns `tacetd` detached in the background and polls `GET /v1/status` until ready.
2. **Fail-Fast Guarantee:** In `tacet run` and `tacet inject`, if `POST /v1/resolve` fails for **any** URI (locked, missing, ambiguous), abort with a non-zero exit code before spawning `<cmd>`.
3. **Output Masking (`tacet run`):** Collect all resolved secret values (`length >= 4` bytes) and redact occurrences in child `stdout` and `stderr` with `[TACET_MASKED]` in real time.

---

## 8. MVP Implementation Roadmap

* **Phase 1: Core Crypto & Upstream Bitwarden Client (`crypto`, `upstream`)**
* Implement Bitwarden `prelogin`, `connect/token`, `refresh_token`, and `sync` HTTP client (supporting custom Vaultwarden URLs).
* Implement PBKDF2/Argon2id `MasterKey`, `MasterPasswordHash`, HKDF stretching, and `Type 2` / `Type 4` CipherString decryption with unit tests.
* Implement local persistence for session metadata and encrypted `vault.cache`.


* **Phase 2: URI Parser & Vault Resolver (`uri`, `resolver`)**
* Implement `TacetUri` parser (`scope`, `item`, `field`, `modifiers`).
* Implement vault lookup, folder/organization scoping, disambiguation errors, and TOTP generation.


* **Phase 3: `tacetd` Axum HTTP-over-UDS Server (`daemon`)**
* Bind `axum::Router` to `tokio::net::UnixListener` at `$XDG_RUNTIME_DIR/tacet/tacetd.sock` (`0600`).
* Implement `/v1/status`, `/v1/login`, `/v1/unlock`, `/v1/lock`, `/v1/logout`, `/v1/sync`, and `/v1/resolve`.
* Implement peer UID check, `Zeroizing` key custody, and idle auto-lock background task.


* **Phase 4: CLI Client & Stream Redaction (`cli`)**
* Implement UDS HTTP client wrapper with automatic `tacetd` background spawning.
* Implement all CLI commands (`login`, `unlock`, `lock`, `logout`, `status`, `sync`, `read`, `inject`, `run` with `.env` parsing and real-time `stdout`/`stderr` masking).
