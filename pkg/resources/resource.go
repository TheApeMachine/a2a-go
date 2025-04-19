package resources

// NOTE: this file purposefully keeps dependencies to the Go standard library
// only.  It mirrors the structure that was prototyped in MCPAddons.txt but
// strips out any logging helpers (errnie) or external packages so it can live
// happily inside a2a‑go.

import (
    "context"
    "encoding/base64"
    "fmt"
    "io"
    "mime"
    "path/filepath"
    "strings"
)

// ResourceType is either text (UTF‑8) or binary (any other mime‑type).
type ResourceType string

const (
    TextResource   ResourceType = "text"
    BinaryResource ResourceType = "binary"
)

// Resource describes a single item that can be fetched via resources/read.
type Resource struct {
    URI         string       `json:"uri"`
    Name        string       `json:"name"`
    Description string       `json:"description,omitempty"`
    MimeType    string       `json:"mimeType,omitempty"`
    Type        ResourceType `json:"type"`
}

// ResourceTemplate allows dynamic resource discovery.  Any variable in curly
// braces is extracted by ParseTemplateVariables.
type ResourceTemplate struct {
    URITemplate string             `json:"uriTemplate"`
    Name        string             `json:"name"`
    Description string             `json:"description,omitempty"`
    MimeType    string             `json:"mimeType,omitempty"`
    Type        ResourceType       `json:"type"`
    Variables   []TemplateVariable `json:"variables,omitempty"`
}

// ResourceContent represents either text or binary data.  Binary is base64
// encoded to fit nicely in JSON‑RPC responses.
type ResourceContent struct {
    URI      string `json:"uri"`
    MimeType string `json:"mimeType,omitempty"`
    Text     string `json:"text,omitempty"`
    Blob     string `json:"blob,omitempty"`
}

// ResourceManager is the high‑level contract that the JSON‑RPC layer will use.
type ResourceManager interface {
    List(ctx context.Context) ([]Resource, []ResourceTemplate, error)
    Read(ctx context.Context, uri string) ([]ResourceContent, error)
    Subscribe(ctx context.Context, uri string) error
    Unsubscribe(ctx context.Context, uri string) error
}

// ----- Helpers / constructor functions -----

// NewResource builds a Resource with optional helpers.
func NewResource(uri, name string, opts ...ResourceOption) *Resource {
    r := &Resource{URI: uri, Name: name}

    if ext := filepath.Ext(uri); ext != "" {
        if mt := mime.TypeByExtension(ext); mt != "" {
            r.MimeType = mt
        }
    }

    // default classification
    if strings.HasPrefix(r.MimeType, "text/") || r.MimeType == "" {
        r.Type = TextResource
    } else {
        r.Type = BinaryResource
    }

    for _, f := range opts {
        f(r)
    }
    return r
}

type ResourceOption func(*Resource)

func WithDescription(desc string) ResourceOption {
    return func(r *Resource) { r.Description = desc }
}

func WithMimeType(mt string) ResourceOption {
    return func(r *Resource) {
        r.MimeType = mt
        if strings.HasPrefix(mt, "text/") {
            r.Type = TextResource
        } else {
            r.Type = BinaryResource
        }
    }
}

// NewResourceContent reads from an io.Reader and returns properly encoded
// ResourceContent.  Errors if the reader cannot be fully consumed.
func NewResourceContent(uri string, r io.Reader, mimeType string) (*ResourceContent, error) {
    data, err := io.ReadAll(r)
    if err != nil {
        return nil, fmt.Errorf("read resource content: %w", err)
    }

    rc := &ResourceContent{URI: uri, MimeType: mimeType}
    if strings.HasPrefix(mimeType, "text/") || mimeType == "" {
        rc.Text = string(data)
    } else {
        rc.Blob = base64.StdEncoding.EncodeToString(data)
    }
    return rc, nil
}

// TemplateVariable is a single {var} placeholder inside a URITemplate.
type TemplateVariable struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Required    bool   `json:"required"`
}

// ----- Template utilities -----

// ParseTemplateVariables returns variables in lexical order of appearance.
func ParseTemplateVariables(tmpl string) []TemplateVariable {
    vars := []TemplateVariable{}
    parts := strings.Split(tmpl, "{")
    for i := 1; i < len(parts); i++ {
        seg := parts[i]
        if idx := strings.Index(seg, "}"); idx != -1 {
            vars = append(vars, TemplateVariable{Name: seg[:idx], Required: true})
        }
    }
    return vars
}
