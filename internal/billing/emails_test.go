package billing

import (
	"reflect"
	"testing"
)

func TestNormalizeEmails(t *testing.T) {
	got, err := NormalizeEmails([]byte(`[" Accounts@Example.com ","accounts@example.COM","other@example.com"]`))
	if err != nil || !reflect.DeepEqual(got, []string{"accounts@example.com", "other@example.com"}) {
		t.Fatalf("normalized = %v, error = %v", got, err)
	}
	for _, raw := range []string{`null`, `[]`} {
		got, err := NormalizeEmails([]byte(raw))
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("empty override %s: %v, %v", raw, got, err)
		}
	}
	for _, raw := range []string{`"a@example.com"`, `true`, `{}`, `[1]`, `[null]`, `[""]`, `["no-at-sign"]`, `["Name <a@example.com>"]`, `["a@example.com\r\nBcc: b@example.com"]`} {
		if got, err := NormalizeEmails([]byte(raw)); err == nil {
			t.Fatalf("accepted %s: %v", raw, got)
		}
	}
}
