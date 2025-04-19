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

## 6  Unified Memory System

A2A-Go provides a unified long-term memory system that combines vector and graph stores for AI agents.

In-memory implementation (no external databases needed):
```bash
go run ./examples/memory-store
```

External databases implementation (uses Qdrant and Neo4j):
```bash
# Start the databases with Docker Compose
docker-compose -f docker-compose.memory.yml up -d

# Set your OpenAI API key
export OPENAI_API_KEY=sk-...

# Run the example
go run ./examples/memory-external
```

For more details on the memory system architecture, see [Memory Architecture](memory-architecture.md).

### Setting Up Qdrant and Neo4j

For production use, you'll want to use real vector and graph databases instead of the in-memory implementations.

#### Qdrant Setup (Vector Store)

1. Run Qdrant using Docker:
   ```bash
   docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
   ```

2. Connect to Qdrant in your code:
   ```go
   embeddingService := memory.NewOpenAIEmbeddingService(os.Getenv("OPENAI_API_KEY"))
   vectorStore := memory.NewQdrantVectorStore("http://localhost:6333", "memories", embeddingService)
   ```

#### Neo4j Setup (Graph Store)

1. Run Neo4j using Docker:
   ```bash
   docker run -p 7474:7474 -p 7687:7687 -e NEO4J_AUTH=neo4j/password neo4j:latest
   ```

2. Connect to Neo4j in your code:
   ```go
   graphStore := memory.NewNeo4jGraphStore("http://localhost:7474", "neo4j", "password")
   ```

### Using the Unified Memory System in Your Code

```go
// Initialize the memory system components
embeddingService := memory.NewOpenAIEmbeddingService(openaiClient) // or memory.NewMockEmbeddingService() for testing
vectorStore := memory.NewQdrantVectorStore("http://localhost:6333", "memories", embeddingService)
graphStore := memory.NewNeo4jGraphStore("http://localhost:7474", "neo4j", "password")
unifiedStore := memory.NewUnifiedStore(embeddingService, vectorStore, graphStore)

// Store a memory
id, err := unifiedStore.StoreMemory(ctx, "Important information to remember", 
    map[string]any{"topic": "knowledge", "importance": 8}, "knowledge")

// Create relationships between memories
err = unifiedStore.CreateRelation(ctx, sourceID, targetID, "related_to", 
    map[string]any{"strength": 0.7})

// Retrieve a memory by ID
memory, err := unifiedStore.GetMemory(ctx, id)

// Search for semantically similar memories
searchParams := memory.SearchParams{
    Query:       "vector databases for AI memory",
    Limit:       10,
    Types:       []string{"knowledge", "concept"},
    Filters:     []memory.Filter{{Field: "topic", Operator: "eq", Value: "memory"}},
}
results, err := unifiedStore.SearchSimilar(ctx, searchParams.Query, searchParams)

// Find related memories through graph relationships
related, err := unifiedStore.FindRelated(ctx, id, []string{"related_to"}, 10)
```

---

## 7  Next Steps

* Expose a file via the Resource manager (`resources/list`).
* Export `OPENAI_API_KEY` to enable real LLM completions.
* Continue with the deep‑dives:
  * [Prompt Kitchen](prompts.md)
  * [Resource Pantry](resources.md)
  * [Sampling Lab](sampling.md)

Happy experiments! 🎈