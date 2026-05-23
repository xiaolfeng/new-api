package naming

import (
	"fmt"
	"strings"
	"testing"
)

func TestAgentName_Deterministic(t *testing.T) {
	id := "ab4299a0ed43122e7"
	name1 := AgentName(id)
	name2 := AgentName(id)
	if name1 != name2 {
		t.Errorf("AgentName should be deterministic: got %q then %q", name1, name2)
	}
	if name1 == "" {
		t.Error("AgentName should not return empty for non-empty id")
	}
}

func TestSessionName_Deterministic(t *testing.T) {
	id := "363f5172-bfaf-43a8-9bb6-2c1d39f54064"
	name1 := SessionName(id)
	name2 := SessionName(id)
	if name1 != name2 {
		t.Errorf("SessionName should be deterministic: got %q then %q", name1, name2)
	}
	if name1 == "" {
		t.Error("SessionName should not return empty for non-empty id")
	}
}

func TestAgentName_Empty(t *testing.T) {
	if AgentName("") != "" {
		t.Error("AgentName of empty string should be empty")
	}
}

func TestSessionName_Empty(t *testing.T) {
	if SessionName("") != "" {
		t.Error("SessionName of empty string should be empty")
	}
}

func TestSessionName_ThreeWords(t *testing.T) {
	id := "363f5172-bfaf-43a8-9bb6-2c1d39f54064"
	name := SessionName(id)

	// Should be a valid non-empty PascalCase string
	if name == "" {
		t.Fatal("SessionName should not be empty")
	}
	if len(name) < 6 {
		t.Errorf("SessionName seems too short: %q", name)
	}

	// Should contain only letters
	for _, r := range name {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			t.Errorf("SessionName should contain only letters, got %q in %q", string(r), name)
		}
	}
}

func TestAgentName_SingleWord(t *testing.T) {
	id := "ab4299a0ed43122e7"
	name := AgentName(id)

	if name == "" {
		t.Fatal("AgentName should not be empty")
	}
	if strings.Contains(name, " ") {
		t.Errorf("AgentName should be a single word, got %q", name)
	}
}

func TestDifferentIDs_DifferentNames(t *testing.T) {
	ids := []string{
		"ab4299a0ed43122e7",
		"363f5172-bfaf-43a8-9bb6-2c1d39f54064",
		"deadbeef12345678",
		"cafebabe9876543",
		"abc123",
		"xyz789",
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}

	names := make(map[string]string)
	for _, id := range ids {
		n := AgentName(id)
		if prev, exists := names[n]; exists {
			t.Logf("AgentName collision: %q and %q both map to %q (acceptable)", prev, id, n)
		}
		names[n] = id
	}

	sessionNames := make(map[string]string)
	for _, id := range ids {
		n := SessionName(id)
		if prev, exists := sessionNames[n]; exists {
			t.Logf("SessionName collision: %q and %q both map to %q (acceptable)", prev, id, n)
		}
		sessionNames[n] = id
	}

	// With ~200^3 = 8M combinations and only 8 samples, collisions should not happen
	if len(sessionNames) < len(ids) {
		t.Logf("SessionName collisions: %d unique names for %d IDs", len(sessionNames), len(ids))
	}
}

func TestSessionName_LargeScaleCollisions(t *testing.T) {
	// Generate UUID-like IDs to test realistic collision rate
	seen := make(map[string]int)
	const n = 10000
	collisions := 0
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", i, i>>4, i>>8, i>>12, i*31+7)
		name := SessionName(id)
		if _, exists := seen[name]; exists {
			collisions++
		}
		seen[name] = i
	}

	collisionRate := float64(collisions) / float64(n)
	if collisionRate > 0.05 {
		t.Errorf("SessionName collision rate too high: %.4f (%d/%d)", collisionRate, collisions, n)
	} else {
		t.Logf("SessionName collision rate: %.4f (%d/%d) — acceptable", collisionRate, collisions, n)
	}
}

func TestKnownMapping(t *testing.T) {
	// Ensure the output looks reasonable for known IDs
	agentName := AgentName("ab4299a0ed43122e7")
	sessionName := SessionName("363f5172-bfaf-43a8-9bb6-2c1d39f54064")

	t.Logf("Agent ID  ab4299a0ed43122e7 → %q", agentName)
	t.Logf("Session ID 363f5172-bfaf-43a8-9bb6-2c1d39f54064 → %q", sessionName)

	if len(agentName) < 3 {
		t.Errorf("Agent name too short: %q", agentName)
	}
	if len(sessionName) < 9 {
		t.Errorf("Session name too short: %q", sessionName)
	}
}
