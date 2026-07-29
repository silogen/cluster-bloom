//go:build linux

package runtime

import (
	"reflect"
	"testing"
)

func TestParseLonghornISCSISessions(t *testing.T) {
	input := `tcp: [1] 169.254.2.2:3260,1 iqn.2015-02.oracle.boot:uefi
tcp: [3] 10.43.11.22:3260,1 iqn.2019-10.io.longhorn:pvc-abc (non-flash)
tcp: [4] 10.43.11.23:3260,1 iqn.2019-10.io.longhorn:pvc-def
malformed
tcp: [5] 10.43.11.24:3260,1 iqn.2000-01.example:other`

	want := []iscsiSession{
		{sid: "3", portal: "10.43.11.22:3260", target: "iqn.2019-10.io.longhorn:pvc-abc"},
		{sid: "4", portal: "10.43.11.23:3260", target: "iqn.2019-10.io.longhorn:pvc-def"},
	}

	if got := parseLonghornISCSISessions(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLonghornISCSISessions() = %#v, want %#v", got, want)
	}
}

func TestParseLonghornISCSISessionsPreservesDuplicateTargetSessions(t *testing.T) {
	input := `tcp: [7] 10.43.11.22:3260,1 iqn.2019-10.io.longhorn:pvc-abc
tcp: [8] 10.43.11.22:3260,1 iqn.2019-10.io.longhorn:pvc-abc`

	got := parseLonghornISCSISessions(input)
	if len(got) != 2 {
		t.Fatalf("parseLonghornISCSISessions() returned %d sessions, want 2", len(got))
	}
	if got[0].sid != "7" || got[1].sid != "8" {
		t.Fatalf("session IDs = %q, %q; want 7, 8", got[0].sid, got[1].sid)
	}
}

func TestParseLonghornISCSISessionsNoMatches(t *testing.T) {
	input := `tcp: [1] 169.254.2.2:3260,1 iqn.2015-02.oracle.boot:uefi`
	if got := parseLonghornISCSISessions(input); len(got) != 0 {
		t.Fatalf("parseLonghornISCSISessions() = %#v, want no sessions", got)
	}
}
