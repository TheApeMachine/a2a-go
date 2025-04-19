# 🧑‍🍳 A2A‑Go – Build delightful AI agents in Go  

> _“Always have something cooking!”_

![A2A‑Go](a2a-go.png)

**a2a‑go** is a reference Go implementation of the **Agent‑to‑Agent (A2A)**
protocol by [Google](https://google.github.io/A2A/#/) plus a growing toolbox
of goodies that make it trivial to stand up a fully‑featured AI agent:

- 🔌 **JSON‑RPC 2.0** server with pluggable method table.
- 📡 **Server‑Sent Events (SSE)** broker for real‑time streaming updates.
- 🧠 Built‑in integration with **OpenAI** (function calling & streaming).
- 📜 **MCP bridge** — exposes your agent’s prompts, resources, roots &
  sampling capabilities through the [Model Context Protocol](https://modelcontextprotocol.io).
- 🔧 A curated set of **tools** (browser, Docker, GitHub, memory, …) ready for
  LLM function‑calling.

The repo is designed for **learning by doing**. Every concept is accompanied
by a runnable example or a “recipe” so you can see something working within
minutes.

---

## Quick Start (5 min)

### 1  Install & build

```bash
git clone https://github.com/theapemachine/a2a-go
cd a2a-go
go run ./examples/basic-agent     # 🗣️  starts an echo‑agent on :8080
```

### 2  Poke it with curl

```bash
# list default prompts
curl -s -X POST localhost:8080/rpc \
  -d '{"jsonrpc":"2.0","id":1,"method":"prompts/list"}' | jq

# send a task (agent echoes the first text part)
curl -s -X POST localhost:8080/rpc \
  -d '{"jsonrpc":"2.0","id":2,"method":"tasks/send","params":{"id":"t1","sessionId":"s1","message":{"role":"user","parts":[{"type":"text","text":"Hello!"}]}}}' | jq

# get task with history
curl -s -X POST localhost:8080/rpc \
  -d '{"jsonrpc":"2.0","id":3,"method":"tasks/get","params":{"id":"t1","historyLength":1}}' | jq

# configure push notifications
curl -s -X POST localhost:8080/rpc \
  -d '{"jsonrpc":"2.0","id":4,"method":"tasks/pushNotification/set","params":{"id":"t1","pushNotificationConfig":{"url":"https://example.com/notify"}}}' | jq
```

### 3  Turn on OpenAI‑power ⚡️

```bash
export OPENAI_API_KEY=sk‑…
go run ./examples/basic-agent   # same command as before

# now ask the agent something fun via sampling/createMessage
curl -s -X POST localhost:8080/rpc \
  -d '{"jsonrpc":"2.0","id":3,"method":"sampling/createMessage","params":{"systemPrompt":"You are a pirate 🤖☠️","temperature":0.9,"messages":[]}}' | jq
```

---

## Features at a Glance

| Area                  | Highlights                                                                   |
| --------------------- | ---------------------------------------------------------------------------- |
| **A2A Core**          | tasks/send, tasks/get, tasks/cancel, push notifications                       |
| **Streaming**         | tasks/sendSubscribe, tasks/resubscribe, SSE broker                            |
| **Session Support**   | Session tracking, message history, history retrieval                          |
| **Push Notifications**| Configure and retrieve push notification settings, JWT token authentication   |
| **Prompts**           | Single or multi‑step prompts, list & fetch via MCP                           |
| **Resources**         | Static files or dynamic URI templates, live subscribe                        |
| **Roots**             | Named root URIs to logically group resources                                 |
| **Sampling**          | Echo stub _or_ real OpenAI completions (auto‑switch)                         |
| **Tools**             | Browser (Rod), Docker exec, GitHub search, Qdrant, Memory store…             |

---

## Learn More 🍽️

Ready to cook something tasty? Pick a recipe and dive right in:

1. 🥄 [Quick Recipes](docs/quickstart.md) — hello world, prompts & streaming.
2. 🧑‍🍳 [Prompt Kitchen](docs/prompts.md) — craft single & multi‑step prompts.
3. 🛍️ [Resource Pantry](docs/resources.md) — expose data & subscribe to updates.
4. ⚗️ [Sampling Lab](docs/sampling.md) — plug in OpenAI or keep it local.

Each deep‑dive ends with a _“What’s next?”_ section so you always have the next
idea to try.

Enjoy & happy hacking! Contributions, issues and recipe ideas are **very**
welcome. 💛
