# Quick Recipes 🍳

> **Goal:** Run an agent, call it, stream results – all in less than ten minutes.

---

## 1  Hello Echo

Start the built‑in echo agent:

```bash
go run ./examples/basic-agent
# ➜  Listening on :8080
```

Send a task (JSON‑RPC):

```bash
curl -s -X POST localhost:8080/rpc \
  -d '{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"id":"t1","message":{"role":"user","parts":[{"type":"text","text":"Ping"}]}}}' | jq .artifacts[0].parts[0].text

# "Ping"
```

🎉 Congratulations – you just used the A2A protocol.

---

## 2  Listing Prompts

```bash
curl -s -X POST localhost:8080/rpc -d '{"jsonrpc":"2.0","id":2,"method":"prompts/list"}' | jq .prompts
```

Fetch a prompt’s full content:

```bash
curl -s -X POST localhost:8080/rpc -d '{"jsonrpc":"2.0","id":3,"method":"prompts/get","params":{"name":"Greeting"}}' | jq .messages[0].content.text
```

---

## 3  Streaming with SSE

The SSE endpoint lives at `/events`.

```bash
# in a second terminal
curl -sN localhost:8080/events | jq -c
```

Back in the first terminal send a streaming request

```bash
curl -s -X POST localhost:8080/rpc \
  -d '{"jsonrpc":"2.0","id":4,"method":"sampling/createMessageStream","params":{"systemPrompt":"You are a poet.","messages":[]}}'
```

Tokens will appear live in the SSE stream.

---

## 4  Next Steps

* Expose a file via the Resource manager (`resources/list`).
* Export `OPENAI_API_KEY` to enable real LLM completions.
* Continue with the deep‑dives:
  * [Prompt Kitchen](prompts.md)
  * [Resource Pantry](resources.md)
  * [Sampling Lab](sampling.md)

Happy experiments! 🎈
