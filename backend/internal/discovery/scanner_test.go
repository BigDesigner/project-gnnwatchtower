package discovery

import (
	"strings"
	"testing"
)

func TestScanSubnetBounds(t *testing.T) {
	// 1. IPv6 should be rejected
	_, err := ScanSubnet("2001:db8::/32")
	if err == nil || !strings.Contains(err.Error(), "IPv4") {
		t.Fatalf("expected IPv6 rejection error, got %v", err)
	}

	// 2. Subnet larger than /22 (e.g. /16 = 65536 hosts) must be rejected before memory allocation
	_, err = ScanSubnet("10.0.0.0/16")
	if err == nil || !strings.Contains(err.Error(), "en fazla /22") {
		t.Fatalf("expected /16 rejection error, got %v", err)
	}

	// 3. Subnet larger than /22 (e.g. /21 = 2048 hosts) must be rejected
	_, err = ScanSubnet("192.168.0.0/21")
	if err == nil || !strings.Contains(err.Error(), "en fazla /22") {
		t.Fatalf("expected /21 rejection error, got %v", err)
	}

	// 4. Invalid CIDR format
	_, err = ScanSubnet("invalid-cidr")
	if err == nil || !strings.Contains(err.Error(), "geçersiz CIDR") {
		t.Fatalf("expected invalid format error, got %v", err)
	}
}
