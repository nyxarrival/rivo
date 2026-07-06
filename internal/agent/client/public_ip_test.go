package client

import "testing"

func TestNormalizePublicIPv6(t *testing.T) {
	got, err := normalizePublicIP("2001:4860:4860::8888", false)
	if err != nil {
		t.Fatalf("normalize public ipv6: %v", err)
	}
	if got != "2001:4860:4860::8888" {
		t.Fatalf("ipv6 = %q, want %q", got, "2001:4860:4860::8888")
	}
}

func TestNormalizePublicIPv6WithBrackets(t *testing.T) {
	got, err := normalizePublicIP("[2001:4860:4860::8888]", false)
	if err != nil {
		t.Fatalf("normalize bracketed public ipv6: %v", err)
	}
	if got != "2001:4860:4860::8888" {
		t.Fatalf("ipv6 = %q, want %q", got, "2001:4860:4860::8888")
	}
}

func TestNormalizePublicIPv6RejectsPrivate(t *testing.T) {
	if _, err := normalizePublicIP("fd00::1", false); err == nil {
		t.Fatal("normalize private ipv6 succeeded, want error")
	}
}

func TestNormalizePublicIPv4RejectsPrivate(t *testing.T) {
	if _, err := normalizePublicIP("192.168.1.1", true); err == nil {
		t.Fatal("normalize private ipv4 succeeded, want error")
	}
}
