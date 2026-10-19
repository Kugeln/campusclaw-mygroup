// Package mcp contains infrastructure only. It registers no HTTP/business routes.
package mcp

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

var Invalid = errors.New("invalid MCP configuration or parameters")
var Denied = errors.New("MCP target not allowed")
var CredentialError = errors.New("MCP credential unavailable")
var RemoteError = errors.New("MCP remote operation failed")
var AuthFailed = errors.Join(RemoteError, errors.New("MCP authentication failed"))
var Timeout = errors.New("MCP operation cancelled or timed out")

type Envelope struct {
	Cipher, Nonce []byte
	KeyID         string
}
type Vault struct {
	active string
	keys   map[string]cipher.AEAD
}

func NewVault(active string, keys map[string][]byte) (*Vault, error) {
	v := &Vault{active: active, keys: map[string]cipher.AEAD{}}
	for id, key := range keys {
		if id == "" || len(id) > 64 || len(key) != 32 {
			return nil, CredentialError
		}
		block, e := aes.NewCipher(key)
		if e != nil {
			return nil, CredentialError
		}
		g, e := cipher.NewGCM(block)
		if e != nil {
			return nil, CredentialError
		}
		v.keys[id] = g
	}
	if v.keys[active] == nil {
		return nil, CredentialError
	}
	return v, nil
}
func (v *Vault) Seal(token string, binding string) (Envelope, error) {
	if v == nil || v.keys[v.active] == nil {
		return Envelope{}, CredentialError
	}
	g := v.keys[v.active]
	nonce := make([]byte, g.NonceSize())
	if _, e := rand.Read(nonce); e != nil {
		return Envelope{}, CredentialError
	}
	return Envelope{Cipher: g.Seal(nil, nonce, []byte(token), []byte(binding)), Nonce: nonce, KeyID: v.active}, nil
}
func (v *Vault) Open(e Envelope, binding string) (string, error) {
	if v == nil {
		return "", CredentialError
	}
	g := v.keys[e.KeyID]
	if g == nil || len(e.Nonce) != g.NonceSize() {
		return "", CredentialError
	}
	plain, err := g.Open(nil, e.Nonce, e.Cipher, []byte(binding))
	if err != nil {
		return "", CredentialError
	}
	return string(plain), nil
}

type Target struct {
	Scheme string   `json:"scheme"`
	Host   string   `json:"host"`
	Port   int      `json:"port"`
	CIDRs  []string `json:"cidrs"`
}
type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type Policy struct {
	targets  []Target
	resolver Resolver
}

func NewPolicy(targets []Target, resolver Resolver) (*Policy, error) {
	if len(targets) == 0 {
		return nil, Denied
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	copyTargets := []Target{}
	for _, t := range targets {
		if (t.Scheme != "http" && t.Scheme != "https") || t.Host == "" || t.Host != strings.ToLower(t.Host) || strings.ContainsAny(t.Host, "/@?#% \\*") || t.Port < 1 || t.Port > 65535 || len(t.CIDRs) == 0 {
			return nil, Invalid
		}
		for _, s := range t.CIDRs {
			if _, e := netip.ParsePrefix(s); e != nil {
				return nil, Invalid
			}
		}
		t.CIDRs = append([]string{}, t.CIDRs...)
		copyTargets = append(copyTargets, t)
	}
	return &Policy{targets: copyTargets, resolver: resolver}, nil
}
func (p *Policy) URL(base string) (*url.URL, Target, error) {
	u, e := url.Parse(strings.TrimRight(base, "/"))
	if e != nil || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || u.Path != "" || u.Host == "" || len(base) > 512 {
		return nil, Target{}, Denied
	}
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if u.Port() != "" {
		port, e = strconv.Atoi(u.Port())
		if e != nil {
			return nil, Target{}, Denied
		}
	}
	if p != nil {
		for _, t := range p.targets {
			if u.Scheme == t.Scheme && strings.ToLower(u.Hostname()) == t.Host && port == t.Port {
				u.Path = "/mcp"
				return u, t, nil
			}
		}
	}
	return nil, Target{}, Denied
}
func safeIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsValid() && !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast() && !ip.IsLinkLocalMulticast() && ip != netip.MustParseAddr("100.100.100.200") && ip != netip.MustParseAddr("168.63.129.16") && ip != netip.MustParseAddr("fd00:ec2::254")
}
func (p *Policy) Resolve(ctx context.Context, t Target) ([]netip.Addr, error) {
	ips, e := p.resolver.LookupNetIP(ctx, "ip", t.Host)
	if e != nil || len(ips) == 0 {
		return nil, Denied
	}
	for i, ip := range ips {
		ip = ip.Unmap()
		if !safeIP(ip) {
			return nil, Denied
		}
		allowed := false
		for _, s := range t.CIDRs {
			prefix, _ := netip.ParsePrefix(s)
			if prefix.Contains(ip) {
				allowed = true
			}
		}
		if !allowed {
			return nil, Denied
		}
		ips[i] = ip
	}
	return ips, nil
}

// Validate performs both URL and current DNS checks. Dialing repeats these checks.
func (p *Policy) Validate(ctx context.Context, base string) error {
	_, t, e := p.URL(base)
	if e != nil {
		return e
	}
	_, e = p.Resolve(ctx, t)
	return e
}

type Config struct {
	Enabled bool
	Vault   *Vault
	Policy  *Policy
}

func LoadConfig(get func(string) string) (Config, error) {
	if get("MCP_ENABLED") != "true" {
		return Config{}, nil
	}
	key, e := base64.StdEncoding.DecodeString(get("MCP_MASTER_KEY"))
	if e != nil {
		return Config{}, CredentialError
	}
	v, e := NewVault(get("MCP_KEY_ID"), map[string][]byte{get("MCP_KEY_ID"): key})
	if e != nil {
		return Config{}, e
	}
	var targets []Target
	if json.Unmarshal([]byte(get("MCP_ALLOWED_TARGETS")), &targets) != nil {
		return Config{}, Invalid
	}
	p, e := NewPolicy(targets, nil)
	return Config{Enabled: true, Vault: v, Policy: p}, e
}
