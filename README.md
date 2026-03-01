<p align="center">
  <h1 align="center">CommandGrid</h1>
  <p align="center"><strong>Production infrastructure for autonomous AI systems.</strong></p>
</p>

<p align="center">
  <img src="./docs/assets/landing-hero-dark.png" alt="CommandGrid hero banner" width="900" />
</p>

<p align="center">
  A robust toolkit and SDK for building distributed AI agents.
</p>

<p align="center">
  <a href="https://github.com/CommandGrid/CommandGrid">Repository</a>
  ·
  <a href="./docs/README.md">Getting Started</a>
  ·
  <a href="./examples/hello-weather/run.sh">Hello-Weather Demo</a>
  ·
  <a href="https://github.com/CommandGrid">Organization</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-active-2563eb?style=for-the-badge" alt="Status Active" />
  <img src="https://img.shields.io/badge/stack-ai%20agents-0f172a?style=for-the-badge" alt="AI Agents" />
  <img src="https://img.shields.io/badge/runtime-secure-111827?style=for-the-badge" alt="Secure Runtime" />
</p>

---

## Core Repositories

- `CommandGrid` - Control plane and orchestration layer
- `RootFS` - Sandbox runtime image and environment
- `GhostProxy` - Secure gateway and llm traffic proxy for agents
- `FlowSpec` - Workflow specifications and CI logic
- `ToolCore` - Shared tool contracts and specifications
- `JudgementD` -  Reference sandbox agent that executes structured tasks, calls LLMs via GhostProxy, runs MCP tool calls, and returns contract-compliant output
