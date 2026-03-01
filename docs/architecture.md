# Architecture

CommandGrid is the orchestrator for the entire agent sandbox system. It reads configuration, manages secrets, provisions sandboxes, coordinates GhostProxy, starts MCP tool sidecars, and optionally restricts outbound network access. This document covers the full architecture across all services.

## System overview

```mermaid
flowchart TB
    Customer[Customer API] -->|"POST /v1/jobs"| GW[api-gateway]
    User[Developer] -->|"sandbox up"| CP

    subgraph host [Host Machine]
        GW
        CP[CommandGrid]
        SecretStore["Secret Store<br/>(File / Env / AWS SM / Vault)"]
        LLMProxy[GhostProxy :8090]
        AllowProxy[allowlist proxy :3128]
        Docker[Docker Daemon]
    end

    subgraph sandbox [Sandbox Network]
        Entrypoint[entrypoint binary]
        Agent[Agent Process]
        ToolEcho[tool: echo]
        ToolCustom[tool: custom]
        Workspace["/workspace (bind mount)"]
    end

    subgraph cloud [External Services]
        Anthropic[Anthropic API]
        OpenAI[OpenAI API]
        FlyAPI[Fly Machines API]
    end

    CP -->|"read secrets"| SecretStore
    CP -->|"register sessions"| LLMProxy
    CP -->|"create container"| Docker
    CP -->|"or create VM"| FlyAPI
    Docker -->|"start"| Entrypoint
    Entrypoint -->|"syscall.Exec"| Agent
    Agent -->|"LLM calls via session token"| LLMProxy
    Agent -->|"MCP HTTP calls"| ToolEcho
    Agent -->|"MCP HTTP calls"| ToolCustom
    Agent -.->|"outbound HTTP"| AllowProxy
    AllowProxy -.->|"allowed hosts only"| Anthropic
    LLMProxy -->|"swap token, forward"| Anthropic
    LLMProxy -->|"swap token, forward"| OpenAI
    LLMProxy -->|"meter tokens"| LLMProxy
```

## Services

| Service | Role | Runs on |
|---|---|---|
| **CommandGrid** | Orchestrator. Config, secrets, provisioning, tools, network policy. | Host (CLI or HTTP server) |
| **GhostProxy** | Stateless reverse proxy. Token validation, credential injection, token metering. | Host (daemon) |
| **RootFS** | Container image + entrypoint. Env stripping, privilege drop, exec agent. | Inside sandbox |
| **api-gateway** | Customer-facing REST API. Job submission, SSE streaming, billing. | Host (daemon) |
| **tools** | MCP tool sidecar containers. One per tool, on sandbox network. | Inside sandbox network |
| **agent** | Reference agent. LLM calls + tool execution loop. | Inside sandbox |

## Communication flow

```mermaid
sequenceDiagram
    participant Customer
    participant GW as api-gateway
    participant CP as CommandGrid
    participant Store as secret store
    participant Proxy as GhostProxy
    participant Docker as Provisioner
    participant Sandbox as sandbox
    participant Tools as tool sidecars

    Customer->>GW: POST /v1/jobs {prompt, tools}
    GW->>CP: POST /internal/v1/sandboxes

    CP->>Store: Resolve secrets
    CP->>CP: Generate session tokens
    CP->>CP: Merge [env] + env_file

    alt Tools configured
        CP->>Docker: Create sandbox network
        CP->>Docker: Start tool sidecar containers
    end

    CP->>Docker: Create agent container (hardened)
    CP->>Proxy: Register sessions (token -> real key)
    CP->>Docker: Start agent container

    Sandbox->>Proxy: LLM call with session token
    Proxy->>Proxy: Swap token, forward, meter usage
    Sandbox->>Tools: MCP tool calls

    GW->>Proxy: GET /v1/sessions/{token}/usage
    GW->>Customer: GET /v1/usage (billing)
```

## Boot sequence

The `Up` command in `pkg/orchestrator/orchestrator.go` runs these steps:

1. **Resolve secrets.** For each secret in `sandbox.yaml`:
   - `inject` mode: read real value from store, add to env map
   - `proxy` mode: generate session token, add token to env, set provider base URL

2. **Merge environment.** Combine `[env]` table and `env_file` values into the env map.

3. **Set agent config.** `AGENT_COMMAND`, `AGENT_ARGS`, `AGENT_USER`, `AGENT_WORKDIR`.

4. **Set control plane URL.** `CONTROL_PLANE_URL` for the sandbox.

5. **Network allowlist.** If `[network] allowed_hosts` is set, inject `HTTP_PROXY` / `HTTPS_PROXY` pointing at the allowlist proxy.

6. **Build mounts.** Convert `shared_dirs` to bind mount specs.

7. **Create sandbox network.** If tools are configured, create an isolated Docker network.

8. **Start tool sidecars.** Launch each `[[tools]]` container on the sandbox network.

9. **Provision sandbox.** Call the provisioner with the env map, mounts, resource limits, and network ID.

10. **Register proxy sessions.** POST to GhostProxy for each `proxy` mode secret.

11. **Start sandbox.** The entrypoint takes over from here.

If any step fails, the orchestrator rolls back: destroys containers, revokes sessions, removes the network.

## Teardown

The `Down` command stops the container, destroys it, and cleans up tool sidecars and the sandbox network.

## Package structure

```
pkg/
├── config/          # sandbox.yaml parsing + validation
├── secrets/
│   ├── iface.go     # Store interface
│   ├── env.go       # EnvStore (env vars)
│   ├── delegated.go # DelegatedStore (AWS SM / Vault)
│   └── session.go   # Session token generation
├── provisioner/
│   ├── provisioner.go  # Provisioner interface
│   ├── docker.go       # Docker Engine API
│   ├── fly.go          # Fly Machines API
│   └── unikraft.go     # kraft.cloud API
├── orchestrator/
│   └── orchestrator.go # Boot + teardown + tool orchestration
├── allowlist/
│   └── proxy.go     # HTTP CONNECT forward proxy with host allowlisting
├── agent/
│   └── contract.go  # Agent I/O contract types
├── memory/
│   ├── iface.go     # Store interface
│   ├── sqlite.go    # SQLite backend
│   └── postgres.go  # PostgreSQL + pgvector backend
└── customer/
    └── profile.go   # Customer profile + secrets provider config

cmd/
├── up.go           # CLI: sandbox up
├── down.go         # CLI: sandbox down
├── status.go       # CLI: sandbox status
├── serve.go        # HTTP server mode
└── helpers.go
```
