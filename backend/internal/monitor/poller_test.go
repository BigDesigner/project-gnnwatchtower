package monitor

import (
	"testing"
)

func TestIsDisallowedHostOrIP(t *testing.T) {
	blocked := []string{
		"169.254.169.254",
		"http://169.254.169.254",
		"169.254.1.1",
		"metadata.google.internal",
		"instance-data",
		"",
	}

	for _, host := range blocked {
		if !isDisallowedHostOrIP(host) {
			t.Errorf("expected host %q to be blocked by SSRF filter, but it was allowed", host)
		}
	}

	allowed := []string{
		"1.1.1.1",
		"8.8.8.8",
	}

	for _, host := range allowed {
		if isDisallowedHostOrIP(host) {
			t.Errorf("expected host %q to be allowed by SSRF filter, but it was blocked", host)
		}
	}
}
