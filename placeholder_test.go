package goway

import "testing"

func TestReplacePlaceholdersBasic(t *testing.T) {
	got, err := replacePlaceholders("create schema ${schema};", map[string]string{"schema": "app"}, "${", "}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "create schema app;" {
		t.Errorf("got %q, want %q", got, "create schema app;")
	}
}

func TestReplacePlaceholdersIsCaseInsensitive(t *testing.T) {
	// Keys are matched case-insensitively, matching Flyway's CaseInsensitiveMap.
	got, err := replacePlaceholders("set ${MySchema}", map[string]string{"myschema": "app"}, "${", "}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "set app" {
		t.Errorf("got %q, want %q", got, "set app")
	}
}

func TestReplacePlaceholdersUnknownKeyFails(t *testing.T) {
	if _, err := replacePlaceholders("${missing}", map[string]string{"present": "x"}, "${", "}"); err == nil {
		t.Error("expected an error for an unresolved placeholder")
	}
}

func TestReplacePlaceholdersLeavesTextWithoutPlaceholders(t *testing.T) {
	script := "SELECT '$100' AS price;"
	got, err := replacePlaceholders(script, map[string]string{"x": "1"}, "${", "}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != script {
		t.Errorf("got %q, want unchanged %q", got, script)
	}
}

func TestReplacePlaceholdersEmptyMapIsNoop(t *testing.T) {
	script := "${kept}"
	got, err := replacePlaceholders(script, nil, "${", "}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != script {
		t.Errorf("got %q, want unchanged %q", got, script)
	}
}

func TestConfigurationLowercasesPlaceholderKeys(t *testing.T) {
	configuration := Configure().Placeholders(map[string]string{"MixedCase": "value"})
	if _, ok := configuration.placeholders["mixedcase"]; !ok {
		t.Errorf("placeholder keys were not normalized to lower case: %v", configuration.placeholders)
	}
}
