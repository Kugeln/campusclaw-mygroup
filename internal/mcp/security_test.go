package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/netip"
	"testing"
)

type fakeResolver struct {
	ips   []netip.Addr
	calls int
}

func (f *fakeResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	f.calls++
	return append([]netip.Addr{}, f.ips...), nil
}
func TestVault(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	v, e := NewVault("one", map[string][]byte{"one": key})
	if e != nil {
		t.Fatal(e)
	}
	a, e := v.Seal("ephemeral-test-credential", "class:1")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := v.Seal("ephemeral-test-credential", "class:1")
	if bytes.Equal(a.Nonce, b.Nonce) {
		t.Fatal("nonce reused")
	}
	plain, e := v.Open(a, "class:1")
	if e != nil || plain != "ephemeral-test-credential" {
		t.Fatal("roundtrip")
	}
	t.Run("tamper", func(t *testing.T) {
		copy := a
		copy.Cipher = append([]byte{}, a.Cipher...)
		copy.Cipher[0] ^= 1
		if _, e = v.Open(copy, "class:1"); e == nil {
			t.Fatal("tamper accepted")
		}
	})
	t.Run("wrong_key", func(t *testing.T) {
		wrong, _ := NewVault("one", map[string][]byte{"one": bytes.Repeat([]byte{8}, 32)})
		if _, e = wrong.Open(a, "class:1"); e == nil {
			t.Fatal("wrong key accepted")
		}
	})
	t.Run("wrong_id", func(t *testing.T) {
		copy := a
		copy.KeyID = "missing"
		if _, e = v.Open(copy, "class:1"); e == nil {
			t.Fatal("unknown key")
		}
	})
	t.Run("binding", func(t *testing.T) {
		if _, e = v.Open(a, "class:2"); e == nil {
			t.Fatal("cross-class ciphertext accepted")
		}
	})
	t.Run("missing", func(t *testing.T) {
		if _, e = NewVault("", nil); e == nil {
			t.Fatal("missing key")
		}
		if _, e = (*Vault)(nil).Seal("test", "x"); e == nil {
			t.Fatal("nil vault")
		}
	})
	t.Run("config", func(t *testing.T) {
		env := map[string]string{"MCP_ENABLED": "true", "MCP_MASTER_KEY": base64.StdEncoding.EncodeToString(key), "MCP_KEY_ID": "one", "MCP_ALLOWED_TARGETS": `[{"scheme":"http","host":"mcp-demo","port":8090,"cidrs":["10.240.1.0/24"]}]`}
		if _, e = LoadConfig(func(k string) string { return env[k] }); e != nil {
			t.Fatal(e)
		}
		delete(env, "MCP_MASTER_KEY")
		if _, e = LoadConfig(func(k string) string { return env[k] }); e == nil {
			t.Fatal("missing config key")
		}
	})
}
func TestPolicyRejectsUnsafeTargets(t *testing.T) {
	r := &fakeResolver{ips: []netip.Addr{netip.MustParseAddr("10.240.1.7")}}
	p, e := NewPolicy([]Target{{Scheme: "http", Host: "mcp-demo", Port: 8090, CIDRs: []string{"10.240.1.0/24"}}}, r)
	if e != nil {
		t.Fatal(e)
	}
	for _, base := range []string{"file:///etc/passwd", "http://localhost:8090", "http://127.0.0.1:8090", "http://169.254.169.254", "http://other:8090", "http://mcp-demo:80", "http://user:password@mcp-demo:8090", "http://mcp-demo:8090/?x=1", "http://mcp-demo:8090/#x"} {
		t.Run(base, func(t *testing.T) {
			if p.Validate(context.Background(), base) == nil {
				t.Fatal("unsafe URL accepted")
			}
		})
	}
	if p.Validate(context.Background(), "http://mcp-demo:8090/") != nil {
		t.Fatal("allowed target blocked")
	}
	for _, ip := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "169.254.169.254", "100.100.100.200", "168.63.129.16", "0.0.0.0", "224.0.0.1", "10.241.1.7"} {
		t.Run("dns_"+ip, func(t *testing.T) {
			r.ips = []netip.Addr{netip.MustParseAddr(ip)}
			if p.Validate(context.Background(), "http://mcp-demo:8090") == nil {
				t.Fatal("DNS bypass")
			}
		})
	}
	r.ips = []netip.Addr{netip.MustParseAddr("10.240.1.7"), netip.MustParseAddr("127.0.0.1")}
	if p.Validate(context.Background(), "http://mcp-demo:8090") == nil {
		t.Fatal("mixed DNS bypass")
	}
}

func TestPolicyMetadataOverridesAllowlist(t *testing.T) {
	for _, raw := range []string{"169.254.169.254", "100.100.100.200", "168.63.129.16", "fd00:ec2::254"} {
		ip := netip.MustParseAddr(raw)
		bits := 32
		if ip.Is6() {
			bits = 128
		}
		resolver := &fakeResolver{ips: []netip.Addr{ip}}
		policy, err := NewPolicy([]Target{{Scheme: "http", Host: "metadata-test", Port: 8090, CIDRs: []string{netip.PrefixFrom(ip, bits).String()}}}, resolver)
		if err != nil {
			t.Fatal(err)
		}
		if policy.Validate(context.Background(), "http://metadata-test:8090") == nil {
			t.Fatalf("metadata bypassed hard deny: %s", raw)
		}
	}
}
