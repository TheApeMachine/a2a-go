# Sampling Lab ⚗️

Generate model replies locally (echo) **or** with OpenAI – the switch is
automatic.

| Environment | Manager Used |
|-------------|--------------|
| OPENAI_API_KEY *unset* | `sampling.DefaultManager` (echo) |
| OPENAI_API_KEY *set*   | `OpenAISamplingManager` (GPT‑4o) |

---

## 1  Synchronous Reply

```bash
curl -s -X POST :8080/rpc \
  -d '{"jsonrpc":"2.0","id":1,"method":"sampling/createMessage","params":{"systemPrompt":"Tell a joke."}}' | jq .samplingMessage.content.text
```

---

## 2  Streaming Reply

Open two terminals.

**Terminal 1 – listen to SSE**

```bash
curl -sN localhost:8080/events | jq -c
```

**Terminal 2 – fire the request**

```bash
curl -s -X POST :8080/rpc \
  -d '{"jsonrpc":"2.0","id":2,"method":"sampling/createMessageStream","params":{"systemPrompt":"Write a haiku about cheese."}}'
```

The first token comes back through RPC, the rest arrives over the SSE stream.

---

## 3  Custom Preferences

```bash
curl -s -X POST :8080/rpc -d '{
  "jsonrpc":"2.0","id":3,
  "method":"sampling/createMessage",
  "params":{
    "systemPrompt":"You are a pirate.",
    "temperature":1.1,
    "maxTokens":64,
    "stopSequences":["Arrr"]
  }}' | jq .samplingMessage.content.text
```

---

### What Next?

* Combine everything: **prompt → embed resource → sampling → stream**.
* Implement a custom `ToolExecutor` so GPT can call your own Go functions.

Have fun exploring! 🧪
