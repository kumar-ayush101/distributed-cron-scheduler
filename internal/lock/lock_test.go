package lock

import "testing"

func TestHashKey_Deterministic(t *testing.T) {
	a := hashKey("cron:job:42")
	b := hashKey("cron:job:42")
	if a != b {
		t.Fatalf("hashKey not deterministic: %d != %d", a, b)
	}
	if hashKey("cron:job:1") == hashKey("cron:job:2") {
		t.Fatal("different keys should hash differently")
	}
}

func TestRandomToken_UniqueAndSized(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		tok, err := randomToken()
		if err != nil {
			t.Fatalf("randomToken error: %v", err)
		}
		if len(tok) != 32 { // 16 bytes hex-encoded
			t.Fatalf("token length = %d, want 32", len(tok))
		}
		if seen[tok] {
			t.Fatalf("duplicate token generated: %s", tok)
		}
		seen[tok] = true
	}
}

func TestPostgresLocker_Kind(t *testing.T) {
	l := NewPostgresLocker(nil)
	if l.Kind() != "postgres" {
		t.Errorf("Kind() = %q, want postgres", l.Kind())
	}
	if err := l.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}
