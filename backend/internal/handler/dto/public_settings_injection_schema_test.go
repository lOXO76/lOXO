package dto

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestPublicSettingsInjectionPayload_CreationEnabledIsAlwaysPresent(t *testing.T) {
	for _, payload := range []any{
		service.PublicSettingsInjectionPayload{},
		service.PublicSettings{},
		service.SystemSettings{},
		PublicSettings{},
		SystemSettings{},
	} {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		if string(fields[service.SettingKeyCreationEnabled]) != "false" {
			t.Errorf("%T must include creation_enabled=false, got %s", payload, fields[service.SettingKeyCreationEnabled])
		}
	}
}

func TestPublicSettingsInjectionPayload_HeaderLinksIsAlwaysPresent(t *testing.T) {
	for _, payload := range []any{
		service.PublicSettingsInjectionPayload{},
		PublicSettings{},
		SystemSettings{},
	} {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		if _, present := fields[service.SettingKeyHeaderLinks]; !present {
			t.Errorf("%T must include header_links", payload)
		}
	}
}

func TestPublicSettingsInjectionPayload_RewardTasksEnabledIsAlwaysPresent(t *testing.T) {
	for _, payload := range []any{service.PublicSettingsInjectionPayload{}, PublicSettings{}, SystemSettings{}} {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		if string(fields["reward_tasks_enabled"]) != "false" {
			t.Errorf("%T must include reward_tasks_enabled=false, got %s", payload, fields["reward_tasks_enabled"])
		}
	}
}

// TestPublicSettingsInjectionPayload_SchemaDoesNotDrift guarantees the SSR
// injection struct exposes every JSON field consumed by the frontend.
//
// Why this test exists: before we extracted a named PublicSettingsInjectionPayload
// type, the inline struct was manually kept in sync with dto.PublicSettings and
// drifted — ChannelMonitorEnabled / AvailableChannelsEnabled were missing, which
// made the frontend read `undefined` on refresh and hide the "可用渠道" menu
// until the async /api/v1/settings/public round-trip finished.
//
// This test compares the two JSON-tag sets and fails if injection is missing
// any field that dto.PublicSettings exposes. Adding a new feature flag with
// only a DTO entry will fail this test until the injection struct is updated.
//
// Intentional exclusions (fields present on dto.PublicSettings that SSR does
// not need to inject) are listed in `dtoOnlyFields` below with a reason.
func TestPublicSettingsInjectionPayload_SchemaDoesNotDrift(t *testing.T) {
	injection := jsonTags(reflect.TypeOf(service.PublicSettingsInjectionPayload{}))
	dtoKeys := jsonTags(reflect.TypeOf(PublicSettings{}))

	// Fields that legitimately live only on the DTO. Keep tiny; document each.
	dtoOnlyFields := map[string]string{
		// force_email_on_third_party_signup lives on the DTO but is not injected via SSR.
		"force_email_on_third_party_signup": "auth-source default, not a feature flag",
	}

	var missing []string
	for key := range dtoKeys {
		if _, ok := injection[key]; ok {
			continue
		}
		if _, allowed := dtoOnlyFields[key]; allowed {
			continue
		}
		missing = append(missing, key)
	}
	if len(missing) > 0 {
		t.Fatalf("service.PublicSettingsInjectionPayload is missing JSON fields present on dto.PublicSettings: %s\n"+
			"add the field to PublicSettingsInjectionPayload (and GetPublicSettingsForInjection), or "+
			"document the exclusion in dtoOnlyFields with a reason.", strings.Join(missing, ", "))
	}
}

func jsonTags(t reflect.Type) map[string]struct{} {
	out := make(map[string]struct{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}
