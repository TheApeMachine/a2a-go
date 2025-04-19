# Resource Pantry 🛍️

Serve data, docs & live updates to your LLM.

## 1  Static Snacks

```go
rm := resources.NewDefaultManager()

rm.AddResource(resources.Resource{
    URI:"file:///motd.txt", Name:"Message of the Day", MimeType:"text/plain", Type:resources.TextResource,
})
```

```bash
curl -s -X POST :8080/rpc -d '{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"file:///motd.txt"}}'
```

## 2  Dynamic URI Templates

```go
tmpl := resources.ResourceTemplate{URITemplate:"file:///docs/{version}/{page}", Name:"Docs", MimeType:"text/markdown", Type:resources.TextResource}
rm.AddTemplate(tmpl)
```

Requesting `file:///docs/v1/getting-started` will match the template and return
a placeholder with extracted vars – perfect for code generation tutorials.

## 3  Live Subscribe

```go
sub, _ := rm.Subscribe(ctx, "file:///motd.txt")

// later – push an update
rm.NotifySubscribers("file:///motd.txt", resources.ResourceContent{URI:"file:///motd.txt", Text:"🍪"})
```

Your client receives a `resources/updated` notification (via SSE).

---

Build up your pantry, feed the LLM!

* Next stop → [Sampling Lab](sampling.md)
