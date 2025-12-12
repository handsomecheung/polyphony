# polyphony

Configuration and management repository for home network cluster services.

## Setup

### Git Hooks

After cloning the repository, configure Git to use the project's custom hooks:

```bash
git config core.hooksPath infra/git/hooks
```

This enables the pre-commit hook that checks for sensitive keywords defined in `infra/git/blackwords`.
