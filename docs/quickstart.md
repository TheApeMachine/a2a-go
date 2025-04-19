# Quick Recipes 🍳

> **Goal:** Run an agent, call it, stream results – all in less than ten minutes.

---

## 1  Hello Echo

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

🎉 Congratulations – you just used the A2A protocol.

---

## 2  Listing Prompts

```bash
curl -s -X POST localhost:3210/rpc -d '{"jsonrpc":"2.0","id":2,"method":"prompts/list"}' | jq .prompts
```

Fetch a prompt's full content:

```bash
curl -s -X POST localhost:3210/rpc -d '{"jsonrpc":"2.0","id":3,"method":"prompts/get","params":{"name":"Greeting"}}' | jq .messages[0].content.text
```

---

## 3  Streaming with SSE

The SSE endpoint lives at `/events`.

```bash
# in a second terminal
curl -sN localhost:3210/events | jq -c
```

Back in the first terminal send a streaming request

```bash
curl -s -X POST localhost:8080/rpc \
  -d '{"jsonrpc":"2.0","id":4,"method":"sampling/createMessageStream","params":{"systemPrompt":"You are a poet.","messages":[]}}'
```

Tokens will appear live in the SSE stream.

---

## 4  Task Streaming & Resubscription

### Streaming Tasks with tasks/sendSubscribe

Use `tasks/sendSubscribe` to send a task and receive streaming updates via SSE:

```bash
# Make sure your SSE listener is running in another terminal:
# curl -sN localhost:8080/events | jq -c

# Send a streaming task
curl -s -X POST localhost:8080/rpc \
  -d '{
    "jsonrpc":"2.0",
    "id":5,
    "method":"tasks/sendSubscribe",
    "params":{
      "id":"stream-task-1",
      "message":{
        "role":"user",
        "parts":[{"type":"text","text":"Process this request with streaming updates"}]
      }
    }
  }' | jq

# You'll immediately receive a working status and subsequent updates will appear in the SSE stream
```

### Reconnecting with tasks/resubscribe

Use `tasks/resubscribe` to reconnect to an existing task's stream:

```bash
# Reconnect to a previously created task
curl -s -X POST localhost:8080/rpc \
  -d '{
    "jsonrpc":"2.0",
    "id":6,
    "method":"tasks/resubscribe",
    "params":{
      "id":"stream-task-1",
      "historyLength":5
    }
  }' | jq

# You'll receive the current state and artifact of the task
# If historyLength is specified, you'll also get recent message history
```

---

## 5  Push Notifications & History

### Configuring Push Notifications

Set up a callback URL to receive task updates:

```bash
# Configure push notifications for a task
curl -s -X POST localhost:8080/rpc \
  -d '{
    "jsonrpc":"2.0",
    "id":7,
    "method":"tasks/pushNotification/set",
    "params":{
      "id":"stream-task-1",
      "pushNotificationConfig":{
        "url":"https://your-callback-url.com/webhook"
      }
    }
  }' | jq

# The server will send updates to the specified URL as the task progresses
```

### Retrieving Push Notification Settings

Check the current push notification configuration:

```bash
# Get push notification settings for a task
curl -s -X POST localhost:8080/rpc \
  -d '{
    "jsonrpc":"2.0",
    "id":8,
    "method":"tasks/pushNotification/get",
    "params":{
      "id":"stream-task-1"
    }
  }' | jq
```

### Retrieving Task History

Get a task with its message history:

```bash
# Get a task with its recent message history
curl -s -X POST localhost:8080/rpc \
  -d '{
    "jsonrpc":"2.0",
    "id":9,
    "method":"tasks/get",
    "params":{
      "id":"stream-task-1",
      "historyLength":10
    }
  }' | jq

# The response will include up to 10 most recent messages in the history field
```

---

## 6  Next Steps

* Expose a file via the Resource manager (`resources/list`).
* Export `OPENAI_API_KEY` to enable real LLM completions.
* Continue with the deep‑dives:
  * [Prompt Kitchen](prompts.md)
  * [Resource Pantry](resources.md)
  * [Sampling Lab](sampling.md)

Happy experiments! 🎈