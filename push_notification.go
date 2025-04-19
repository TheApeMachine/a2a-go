package a2a

// Very lightweight JWT (RS256) helper and JWK endpoint without external
// dependencies.  We implement just enough to interoperate with typical JWT
// libraries on the receiving side – no fancy claims validation needed on the
// sender.

import (
    "bytes"
    "context"
    "crypto"
    "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "math/big"
    "net/http"
    "time"
)

// --------------------------- helper types -----------------------------------

type jwkKey struct {
    Kty string `json:"kty"`
    Kid string `json:"kid"`
    Use string `json:"use,omitempty"`
    Alg string `json:"alg,omitempty"`
    N   string `json:"n"`
    E   string `json:"e"`
}

type jwkSet struct {
    Keys []jwkKey `json:"keys"`
}

// PushNotificationSenderAuth encapsulates a private key and provides helper
// methods to serve its public JWK and send signed notifications.
type PushNotificationSenderAuth struct {
    key       *rsa.PrivateKey
    kid       string
    jwksJSON  []byte
    httpClient *http.Client
}

// NewPushNotificationSenderAuth generates a fresh 2048‑bit RSA keypair.
func NewPushNotificationSenderAuth() (*PushNotificationSenderAuth, error) {
    pk, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return nil, err
    }
    kid := randomKid()

    pub := pk.PublicKey
    n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
    e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())

    set := jwkSet{Keys: []jwkKey{{
        Kty: "RSA",
        Kid: kid,
        Alg: "RS256",
        Use: "sig",
        N:   n,
        E:   e,
    }}}
    jwkBytes, _ := json.Marshal(set)

    return &PushNotificationSenderAuth{key: pk, kid: kid, jwksJSON: jwkBytes}, nil
}

// JWKSHandler serves the public key set at /.well‑known/jwks.json.
func (p *PushNotificationSenderAuth) JWKSHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write(p.jwksJSON)
    }
}

// VerifyURL performs a simple HEAD request to check reachability.
func (p *PushNotificationSenderAuth) VerifyURL(ctx context.Context, url string) bool {
    req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
    resp, err := p.http().Do(req)
    if err != nil {
        return false
    }
    resp.Body.Close()
    return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// Send serialises data to JSON and posts it to the callback URL with an
// Authorization: Bearer <signed‑jwt> header.
func (p *PushNotificationSenderAuth) Send(ctx context.Context, url string, data interface{}) error {
    token, err := p.signJWT()
    if err != nil {
        return err
    }

    body, _ := json.Marshal(data)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := p.http().Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return errors.New("push notification failed")
    }
    return nil
}

// --------------------------- internal helpers -------------------------------

func (p *PushNotificationSenderAuth) signJWT() (string, error) {
    header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": p.kid}
    claims := map[string]interface{}{
        "iss": "a2a‑go",
        "iat": time.Now().Unix(),
        "exp": time.Now().Add(10 * time.Minute).Unix(),
    }

    h, _ := json.Marshal(header)
    c, _ := json.Marshal(claims)
    b64h := base64.RawURLEncoding.EncodeToString(h)
    b64c := base64.RawURLEncoding.EncodeToString(c)
    signingInput := b64h + "." + b64c

    hash := sha256.Sum256([]byte(signingInput))
    sig, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, hash[:])
    if err != nil {
        return "", err
    }
    token := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
    return token, nil
}

func (p *PushNotificationSenderAuth) http() *http.Client {
    if p.httpClient != nil {
        return p.httpClient
    }
    return http.DefaultClient
}

func randomKid() string {
    b := make([]byte, 6)
    rand.Read(b)
    return base64.RawURLEncoding.EncodeToString(b)
}
