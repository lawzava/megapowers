package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Claude Code 2.1.285 reports cc-plugin-agents-md@builtin in system/init even
// with an empty config, which failed every arm's exact inventory check.
func TestClaudeSettingsDisableBuiltinPlugins(t *testing.T) {
	req := brokerRequest{ActorHome: t.TempDir()}
	path, err := writeClaudeSettings(req)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatal(err)
	}
	// Claude Code 2.1.295 adds cc-plugin-plugin-authoring@builtin the same way.
	for _, builtin := range []string{"cc-plugin-agents-md@builtin", "cc-plugin-plugin-authoring@builtin"} {
		enabled, listed := settings.EnabledPlugins[builtin]
		if !listed || enabled {
			t.Fatalf("enabledPlugins = %v, want %s disabled", settings.EnabledPlugins, builtin)
		}
	}
}

func TestClaudeInventoryErrorNamesUnexpectedPlugins(t *testing.T) {
	builtin := map[string]any{"name": "cc-plugin-agents-md", "path": "builtin", "source": "cc-plugin-agents-md@builtin"}
	_, err := validateClaudeInventory([]any{builtin}, brokerRequest{Arm: "control"})
	if err == nil || !strings.Contains(err.Error(), "cc-plugin-agents-md@builtin") {
		t.Fatalf("control error = %v, want the plugin source named", err)
	}
	candidate := map[string]any{"name": "megapowers", "path": "/plugin", "source": "megapowers@inline"}
	_, err = validateClaudeInventory([]any{candidate, builtin}, brokerRequest{Arm: "treatment", PluginRepo: "/plugin"})
	if err == nil || !strings.Contains(err.Error(), "cc-plugin-agents-md@builtin") {
		t.Fatalf("treatment error = %v, want the plugin source named", err)
	}
}
