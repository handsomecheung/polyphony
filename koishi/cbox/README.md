# cbox

`cbox` runs a workspace container and an Arondo runner on the `nur` Kubernetes
node, using the existing devbox image. The deployment lives in the `others`
namespace and exposes SSH through a ClusterIP service on port 22.

## Configuration and deployment

The host directories declared in `k8s.app.yaml` must be available on `nur`.
They supply the workspaces, sharepoint, repository, and secret configuration.
The box password is rendered from the `koishi.container.cbox` vault entry by
`my-k8s-deploy`.

Create a local `.env` beside `run.arondo-runner.sh` containing
`ARONDO_HOST_SERVER` (the server hostname) and `ARONDO_RUNNER_TOKEN`.
Keep credentials out of version control.

Deploy with the repository's deployment helper available in `PATH`:

```bash
bash deploy.sh
```

## Startup

`entrypoint.root.sh` prepares the box user, runs `entrypoint.box.sh` as that
user, and starts SSH. The box entrypoint runs the available workspace
initializers, sets asdf versions, and loads the Nix profile.

It then starts an interactive zsh with `zsh -ic` so the user's `~/.zshrc`
finishes loading before `run.arondo-runner.sh` runs under Bash. Exported
environment variables, including `PATH`, reach the runner; zsh aliases and
functions do not. Startup configuration must work without a terminal.

The runner script downloads the latest Linux amd64 Arondo runner to
`/tmp/arondo.runner.bin` and starts it in the background, with output written
to `/tmp/arondo.runner.log`.
