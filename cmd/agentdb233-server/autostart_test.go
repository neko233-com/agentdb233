package main

import (
	"strings"
	"testing"
)

func TestAutostartUnitContent(t *testing.T) {
	unit := systemdUnitContent("/usr/local/bin/agentdb233-server", "/var/lib/agentdb233", "127.0.0.1:23390")
	if !strings.Contains(unit, "agentdb233-server") || !strings.Contains(unit, "Restart=on-failure") {
		t.Fatalf("unit=%s", unit)
	}
	plist := launchdPlistContent("/usr/local/bin/agentdb233-server", "/var/lib/agentdb233", "127.0.0.1:23390")
	if !strings.Contains(plist, launchdLabel) || !strings.Contains(plist, "RunAtLoad") {
		t.Fatalf("plist=%s", plist)
	}
}
