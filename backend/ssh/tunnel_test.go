package ssh

import "testing"

func TestSSHAddressSupportsIPv6(t *testing.T) {
	if got := sshAddress("2001:db8::2", 22); got != "[2001:db8::2]:22" {
		t.Fatalf("IPv6 SSH address = %q", got)
	}
	if got := sshAddress("127.0.0.1", 2222); got != "127.0.0.1:2222" {
		t.Fatalf("IPv4 SSH address = %q", got)
	}
}
