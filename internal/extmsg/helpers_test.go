package extmsg

import "testing"

func TestEncodeMetadataFieldsPrefixesMetadataAndSkipsBlankFieldKeys(t *testing.T) {
	meta := map[string]string{
		"channel": "alerts",
	}
	fields := map[string]string{
		"":       "ignored",
		"status": "open",
	}

	got := encodeMetadataFields(meta, fields)

	if got["status"] != "open" {
		t.Fatalf("status = %q, want open", got["status"])
	}
	if got[""] != "" {
		t.Fatalf("blank field key should be omitted, got %q", got[""])
	}
	if got[metadataPrefix+"channel"] != "alerts" {
		t.Fatalf("prefixed metadata = %q, want alerts", got[metadataPrefix+"channel"])
	}
	if got["channel"] != "" {
		t.Fatalf("unprefixed metadata should be omitted, got %q", got["channel"])
	}
	if meta["channel"] != "alerts" {
		t.Fatalf("encodeMetadataFields mutated input metadata: %#v", meta)
	}
}
