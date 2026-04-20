# BWW (BitWarden for Webapp)

BWW (BitWarden for Webapp) is a Go-based application that wraps the Bitwarden CLI (`bw`) to provide a simple HTTP API for retrieving secrets, custom fields, and attachments. It includes a template rendering engine to inject secrets directly into configuration files.

## Features

- **Memory Cache**: Loads all items into memory on startup for fast retrieval.
- **On-demand Sync**: Synchronize with Bitwarden server via API.
- **Template Rendering**: Replace placeholders in files with actual secrets.
- **Attachment Support**: Retrieve binary or Base64-encoded attachments.

## API Endpoints

### 1. Get Password
- **URL**: `GET /{name}/password`
- **Description**: Returns the password for the specified item name.

### 2. Get Custom Field
- **URL**: `GET /{name}/field/{field_name}`
- **Description**: Returns the value of a custom field for the specified item.

### 3. Get Attachment
- **URL**: `GET /{name}/attachment/{filename}`
- **Description**: Downloads the specified attachment file.

### 4. Get Attachment (Base64)
- **URL**: `GET /{name}/attachment/{filename}/base64`
- **Description**: Returns the content of the attachment encoded in Base64 (Plain text).

### 5. Sync Cache
- **URL**: `UPDATE /sync`
- **Method**: `UPDATE`
- **Description**: Triggers `bw sync` and reloads all items into memory.

### 6. Render Template
- **URL**: `POST /render`
- **Method**: `POST`
- **Description**: Accepts a file (binary/text) in the request body and replaces placeholders with Bitwarden data.

## Placeholder Formats for `/render`

| Format | Description | Example |
| :--- | :--- | :--- |
| `__{{name}}__` | Password of item `name` | `__{{koishi.litellm}}__` |
| `__{{name:f:field}}__` | Custom field `field` of item `name` | `__{{infra.common-users:f:hh}}__` |
| `__{{name:a:file}}__` | Content of attachment `file` | `__{{sshkeys:a:id_rsa}}__` |
| `__{{name:a:file:a:b64}}__` | Base64 content of attachment `file` | `__{{sshkeys:a:id_rsa:a:b64}}__` |

## Deployment

### Environment Variables
- `BW_URL`: Bitwarden/Vaultwarden server URL.
- `BW_CLIENTID`: API Client ID.
- `BW_CLIENTSECRET`: API Client Secret.
- `BW_PASSWORD`: Master password to unlock the vault.

### Building and Running
1. Build the image: `./code/build.sh`
2. Deploy to Kubernetes: `my-k8s-deploy --file=k8s.app.yaml` (from root)

## Development
The source code is located in the `code/` directory.
- `main.go`: API implementation and logic.
- `Dockerfile`: Multi-stage build for Go and Bitwarden CLI.
- `entrypoint.sh`: Handles login and unlocking on container startup.
