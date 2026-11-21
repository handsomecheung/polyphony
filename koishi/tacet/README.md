# Tacet

A fast, zero-knowledge CLI secret manager and environment injector compatible with **Bitwarden** and self-hosted **Vaultwarden** servers.

Inspired by the developer ergonomics of 1Password CLI (`op`), Tacet eliminates plaintext `.env` files on disk, avoids secret leakage into shell histories or process tables, and provides sub-5ms secret resolution via a local background daemon.

---

## Architecture

Tacet is built with a decoupled client-daemon architecture:

* **`tacetd` (Stateful Background Daemon):**
  An `axum`-powered HTTP/1.1 REST server listening exclusively over a Unix Domain Socket (`tacetd.sock`). It acts as the single secure custody point for OAuth tokens, encrypted vault cache, and decrypted symmetric keys in memory (with automatic idle auto-lock zeroization).
* **`tacet` (Stateless CLI Client):**
  A fast, lightweight CLI binary that parses templates and flags, communicates with `tacetd` over the Unix socket, injects resolved secrets directly into child process memory, and redacts sensitive strings from `stdout` and `stderr` streams in real time.

```
                      +-----------------------------+
                      | Upstream Bitwarden /        |
                      | Vaultwarden Server          |
                      +--------------+--------------+
                                     ^
                         HTTPS sync  |  OAuth login
                                     v
+------------------------------------+------------------------------------+
|  tacetd (Background Daemon)                                             |
|                                                                         |
|  [Unix Domain Socket: $XDG_RUNTIME_DIR/tacet/tacetd.sock (0600)]        |
|  - Peer UID verification via SO_PEERCRED                                |
|  - In-memory key custody (ZeroizeOnDrop, idle auto-lock timer)          |
|  - Local encrypted storage: ~/.config/tacet/ & ~/.cache/tacet/ (0600)   |
|  - RFC 6238 live TOTP generation & Type 2 / Type 4 decryption           |
+------------------------------------+------------------------------------+
                                     ^
                         HTTP / UDS  |  JSON RPC (<5ms)
                                     v
+------------------------------------+------------------------------------+
|  tacet (Stateless CLI Client)                                           |
|                                                                         |
|  tacet login  |  tacet unlock  |  tacet read  |  tacet inject           |
|  tacet run -- <cmd> (memory injection + real-time stream masking)       |
+-------------------------------------------------------------------------+
```

---

## Features

- **Zero-Knowledge Security:** All cryptographic operations follow Bitwarden's security model. Master passwords and keys are zeroized immediately after use.
- **Lightning Fast (<5ms):** Reads and injects secrets directly from the local daemon's decrypted memory cache without repeated network requests or decryption overhead.
- **Fail-Fast Guarantees:** Aborts process execution before launching child commands if any referenced secret cannot be resolved or is locked.
- **Real-Time Stream Masking:** Transparently monitors child process `stdout` and `stderr` in `tacet run`, masking sensitive values with `[TACET_MASKED]`.
- **Dynamic TOTP Generation:** Live RFC 6238 6-digit TOTP code calculation from Bitwarden authenticator keys.
- **Flexible `tacet://` URIs:** Expressive URI format supporting folders, organizations, collections, custom fields, and modifiers (e.g. `base64`, `json=<path>`, `trim`).

---

## Installation & Building

Prerequisites: Rust 1.80+ (Cargo).

```bash
# Option 1: Build release binaries and install to ~/bin/
./scripts/build.sh

# Option 2: Build with cargo directly
cargo build --release

# The compiled binaries will be located at:
# target/release/tacet
# target/release/tacetd
```

---

## Quickstart

### 1. Authenticate & Initial Sync
Log in to your Bitwarden or Vaultwarden account. If `tacetd` is not already running, `tacet` will automatically spawn it in the background:

```bash
# Official Bitwarden Cloud
tacet login --email user@example.com

# Self-Hosted Vaultwarden
tacet login --server https://vault.example.com --email user@example.com

# API Key Login (Headless / CI/CD)
tacet login --apikey --client-id "user.xxxx" --client-secret "yyyy"
# Or with environment variables: TACET_CLIENT_ID, TACET_CLIENT_SECRET, TACET_PASSWORD
```

### 2. Check Daemon Status
Inspect authentication status, server endpoint, lock state, and cached item counts:

```bash
tacet status
```

### 3. List Vault Items
List items, folders, and URI prefixes in a formatted table or JSON:

```bash
# List all items
tacet list

# Filter by scope or search query
tacet list personal
tacet list -q "github"

# JSON output
tacet list --json
```

Listed URI prefixes preserve `/` in nested folder and collection paths. Append a field target such as `/password` to read a secret.

### 4. Read a Secret
Fetch a single secret to standard output:

```bash
# Print password
tacet read "tacet://personal/GitHub/password"

# Generate live TOTP token
tacet read "tacet://personal/AWS/totp"
```

### 5. Template Injection
Replace `{{ tacet://... }}` placeholders in configuration templates without writing secrets to disk:

```bash
# Render template file to destination
tacet inject -i config.template.json -o config.json

# Stdin / Stdout pipeline
cat template.env | tacet inject > .env
```

### 6. Run Commands with Secret Injection
Pass secrets directly into environment variables with real-time output redaction:

```bash
# Inject secrets from a .env file containing tacet:// references
tacet run --env-file .env -- npm start

# Or execute commands directly
DATABASE_URL="tacet://personal/Database/uri" tacet run -- ./migrate.sh
```

### 7. Lock and Unlock
Lock key custody manually, or restore session using your Master Password:

```bash
# Lock in-memory keys
tacet lock

# Unlock session
tacet unlock
```

---

## `tacet://` URI Specification

Tacet uses a standardized URI format to reference secrets:

```text
tacet://<scope>/<item>/<field>[?<modifiers>]
```

### 1. `<scope>` Rules

| Scope | Target |
|---|---|
| `personal` | Personal vault (`organizationId == null`), ignoring folder hierarchy |
| `personal/<folder>` | Items inside a specific personal folder (nested paths like `folder/subfolder` use `/` directly) |
| `org/<org_name>/<collection>` | Shared items inside an Organization's collection (nested paths like `col1/col2` use `/` directly) |
| `*` | Search all accessible vaults (fails fast if ambiguous) |

### 2. `<item>` Rules

- Case-insensitive search by item `name` (e.g., `GitHub`, `Postgres DB`).
- Supports exact 36-character item `UUID` (e.g., `d3b07384-d113-46fb-9c88-e9f02c61e808`).
- Nested collection and folder paths use `/` directly, e.g. `tacet://org/Default/Network/Koishi/Infra/wireproxy/password`. Existing `%2F`-encoded paths remain supported.
- Paths are parsed from the end: `<item>/<field>` or `<item>/fields/<custom_field>`. The preceding path is the collection or folder. The `fields` segment immediately before the final segment introduces an explicit custom field.
- Spaces can be preserved inside quotes; slashes inside organization, item, and field names must be encoded as `%2F`.
- If multiple items match the query, Tacet returns a disambiguation error listing matching UUIDs and folders.

### 3. `<field>` Mapping

| Bitwarden Field | URI Target | Description |
|---|---|---|
| `login.username` | `username` | Plaintext username |
| `login.password` | `password` | Plaintext password |
| `login.totp` | `totp` | Live 6-digit RFC 6238 TOTP code |
| `login.totp` (raw) | `totp_secret` | Raw TOTP secret / `otpauth://` URI |
| `login.uris[0].uri` | `uri` / `uri.0` | Primary URL (`uri.<n>` for index `n`) |
| `notes` | `notes` | Secure note content |
| `fields[name == X]` | `fields/<X>` or `<X>` | Custom field matching name `X` (falls back if built-in not matched) |

### 4. `[?<modifiers>]` (Optional Query Parameters)

- `encoding=base64`: Encode multi-line secrets or binary values into base64.
- `encoding=hex`: Encode value as lowercase hexadecimal string.
- `json=<dot.path>`: Parse JSON payload and extract field (e.g., `?json=credentials.client_secret`).
- `trim=true|false`: Strip leading/trailing whitespace and newlines (default: `true`).

---

## CLI Reference

### Global Options

```bash
tacet [--socket <path>] <subcommand>
```

### Session Management

```bash
tacet login [--server <url>] [--email <email>] [--apikey] [--client-id <id>] [--client-secret <secret>] [--password <pwd>]
tacet unlock [--password <pwd>]
tacet lock
tacet logout
tacet status
tacet sync
```

### Vault Exploration & Secret Consumption

```bash
# List vault items (table or json)
tacet list [scope] [-q <query>] [--json]

# Output secret value
tacet read <uri> [--no-newline]

# Inject into file templates
tacet inject [-i <template_file>] [-o <output_file>]

# Execute process with injected environment variables
tacet run [--env-file <file>] [--no-mask] -- <command> [args...]
```

---

## Storage & Security Model

- **Configuration:** `~/.config/tacet/session.json` (mode `0600`) stores server URL, email, and user public/private key metadata.
- **Device ID:** `~/.config/tacet/device_id` (mode `0600`) stores a persistent UUIDv4 to identify the client installation to Vaultwarden and Bitwarden servers.
- **Cache:** `~/.cache/tacet/vault.cache` (mode `0600`) caches encrypted CipherStrings directly from Bitwarden.
- **Socket:** `$XDG_RUNTIME_DIR/tacet/tacetd.sock` (mode `0600`, directory `0700`) enforces local peer UID validation via `SO_PEERCRED`. No TCP sockets are bound, preventing DNS-rebinding and browser-based CSRF attacks.
- **Key Custody:** Symmetric keys reside strictly within `tacetd` memory inside zeroizing allocations (`ZeroizeOnDrop`) and are wiped on idle timeout (default: 30 minutes) or explicit lock/logout.

---

## Testing

Run all unit and integration test suites:

```bash
cargo test
```
