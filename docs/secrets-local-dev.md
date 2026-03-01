# Secrets for Local Development

This guide explains how to store and use secrets when developing locally with CommandGrid.

---

## Options Overview

| Provider | Use case | How secrets are stored |
|----------|----------|-------------------------|
| **env** | Default. Env vars or `.env` file | `SECRET_*` env vars or `.env` file |
| **bitwarden** | Bitwarden vault | Bitwarden CLI (`bw`) |

Use `--secrets-provider <name>` on `up`, `down`, `run`, `status`, and `serve`. Default is `env`.

---

## Option 1: Environment Variables (EnvStore)

Use environment variables or a `.env` file. No secrets on disk.

### Env vars

Set `SECRET_` + uppercase secret name. Names use underscores.

| Secret in `sandbox.yaml` | Env var to set |
|--------------------------|----------------|
| `anthropic_key` | `SECRET_ANTHROPIC_KEY` |
| `openai_key` | `SECRET_OPENAI_KEY` |
| `minimax_key` | `SECRET_MINIMAX_KEY` |
| `github_token` | `SECRET_GITHUB_TOKEN` |

```bash
export SECRET_ANTHROPIC_KEY="sk-ant-..."
CommandGrid up --secrets-provider env --config sandbox.yaml
```

### `.env` file

Create a `.env` file in your project directory (add to `.gitignore`). Use the **secret name** as the key (lowercase, underscores):

```bash
# .env - keys must match secret names in sandbox.yaml
anthropic_key=sk-ant-...
openai_key=sk-...
minimax_key=your-minimax-api-key
github_token=ghp_...
```

```bash
CommandGrid up --secrets-provider env --config sandbox.yaml
```

By default, the env provider now resolves a populated `.env` at the repository root and returns a clear error if it is missing/empty. Use `--secrets-dir /path/to/.env` to specify a different file. Env vars override values from the `.env` file.

---

## Option 2: Bitwarden

Use Bitwarden as the secret store. Requires `bw` CLI and an unlocked session.

### Setup

1. Install Bitwarden CLI: `brew install bitwarden-cli` (or [bw CLI docs](https://bitwarden.com/help/cli/))
2. Log in: `bw login`
3. Unlock and export session:
   ```bash
   export BW_SESSION="$(bw unlock --raw)"
   ```

If you see a parse error like `invalid character '?' looking for beginning of value`,
the vault is still locked in that shell. Re-run the unlock command and verify:

```bash
bw status
bw list items --search anthropic_key --session "$BW_SESSION"
```

### How to store secrets in Bitwarden

#### Item type

Use **Login** or **Secure Note**:

- **Login**: Put the secret in the **password** field (preferred).
- **Secure Note**: Put the secret in the **notes** field.

**Do not use** SSH key or Identity items. The integration only reads `login.password` and `notes`.

#### Item name

Use the **exact** secret name from your `sandbox.yaml`. Lowercase with underscores.

| Secret in `sandbox.yaml` | Bitwarden item name |
|--------------------------|---------------------|
| `anthropic_key` | `anthropic_key` |
| `openai_key` | `openai_key` |
| `minimax_key` | `minimax_key` |
| `github_token` | `github_token` |

Do **not** use env-style names like `ANTHROPIC_KEY`. The lookup is by item name, not env var.

#### Example

For:

```yaml
secrets:
  anthropic_key:
    mode: proxy
    env_var: ANTHROPIC_API_KEY
    provider: anthropic
```

Create a Bitwarden item:

- **Name**: `anthropic_key`
- **Type**: Login
- **Password**: `sk-ant-...` (your API key)

Or use a Secure Note:

- **Name**: `anthropic_key`
- **Notes**: `sk-ant-...`

### Run with Bitwarden

```bash
export BW_SESSION="$(bw unlock --raw)"
CommandGrid up --secrets-provider bitwarden --config sandbox.yaml
# or
CommandGrid run --secrets-provider bitwarden
```

`CommandGrid run` manages proxy auth for local development: it restarts GhostProxy and mints a fresh admin token unless you pass `--reuse-proxy`.

`CommandGrid up` now also self-heals once for proxy token drift by restarting GhostProxy and retrying startup.

---

## Summary

| Question | Answer |
|----------|--------|
| Can local dev use env vars? | Yes. Use `--secrets-provider env` and set `SECRET_ANTHROPIC_KEY` etc., or use a `.env` file. |
| Bitwarden: Login or Note? | Either. Login password is preferred; Secure Note notes is fallback. |
| Bitwarden: SSH key item? | No. Use Login or Secure Note. |
| Bitwarden: Use caps like env var? | No. Use exact names from `sandbox.yaml`, e.g. `anthropic_key`. |
