package machine

import (
	"testing"
)

const fixtureVMStat = `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               73352.
Pages active:                            302556.
Pages inactive:                          291440.
Pages speculative:                        27036.
Pages throttled:                              0.
Pages wired down:                        107561.
Pages purgeable:                           3429.
"Translation faults":               30465219410.
Pages copy-on-write:                  187575813.
`

const fixtureSwap = `vm.swapusage: total = 4096.00M  used = 2471.94M  free = 1624.06M  (encrypted)`

const fixtureMemsize = `17179869184`

func TestParseVMStat(t *testing.T) {
	free, inactive, pageSize, err := parseVMStat(fixtureVMStat)
	if err != nil {
		t.Fatalf("parseVMStat() error: %v", err)
	}
	if pageSize != 16384 {
		t.Errorf("pageSize = %d, want 16384", pageSize)
	}
	if free != 73352 {
		t.Errorf("free = %d, want 73352", free)
	}
	if inactive != 291440 {
		t.Errorf("inactive = %d, want 291440", inactive)
	}
}

func TestParseSwap(t *testing.T) {
	total, used, err := parseSwap(fixtureSwap)
	if err != nil {
		t.Fatalf("parseSwap() error: %v", err)
	}
	if total != 4096.0 {
		t.Errorf("total = %f, want 4096.0", total)
	}
	if used != 2471.94 {
		t.Errorf("used = %f, want 2471.94", used)
	}
}

func TestParseMemsize(t *testing.T) {
	total, err := parseMemsize(fixtureMemsize)
	if err != nil {
		t.Fatalf("parseMemsize() error: %v", err)
	}
	if total != 17179869184 {
		t.Errorf("total = %d, want 17179869184", total)
	}
}

func TestParseClaudeCount(t *testing.T) {
	count := parseClaudeCount("3\n")
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestParseClaudeCountZero(t *testing.T) {
	count := parseClaudeCount("0\n")
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestParseClaudeCountEmpty(t *testing.T) {
	count := parseClaudeCount("")
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}
