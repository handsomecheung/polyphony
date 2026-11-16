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

Before deploying the box, generate its kubeconfig once from a machine with
`kubectl`, Python 3, and administrator access to the intended cluster:

```bash
bash setup-kubeconfig.cbox-user.sh
```

This applies `k8s.cbox-user.yaml`, removes the old `cbox-user-binding`
ClusterRoleBinding, and generates a long-lived ServiceAccount token. The
`ck-dev` and `builder` namespaces must already exist. The administrator's
kubectl authentication may prompt during this setup; subsequent cbox kubectl
commands use the generated token directly without an exec authentication script.
The script checks access in both namespaces and rejects unexpected access to
Secrets in `default`, `others`, or `kube-system`, or to Nodes.

The RoleBindings grant full management of namespaced resources in `ck-dev`
and `builder`, including Secrets and namespace RBAC. They do not grant access
to other namespaces or cluster-scoped resources. Other existing RBAC bindings
can grant additional permissions and must be reviewed if setup detects
unexpected access.

The kubeconfig is saved with mode `600`, outside this repository, at
`/mnt/coder-workspaces/private-workspace/others/c/coder-workspaces/private-workspace/kube/config`
on the host. The deployment mounts that directory read-only at
`/home/box/.kube`. Run setup as the OS user whose UID matches the container's
`box` user so the file remains readable by that user. An optional first
argument overrides the output path, and `KUBECTL_BIN` selects the kubectl
executable; a different output directory also requires updating the mount.

Inside cbox:

```bash
kubectl get pods -n ck-dev
kubectl get pods -n builder
```

The default namespace is `ck-dev`. This token has no automatic expiry; protect
the kubeconfig like a password. To revoke it, delete the
`kube-system/cbox-user-token` Secret with administrator credentials. Re-running
setup reuses an existing token; delete the Secret first when rotating it.

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
