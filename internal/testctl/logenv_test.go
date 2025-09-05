package testctl

import (
	"os"
	"testing"
)

func TestEnvStr(t *testing.T) {
	key := "TESTCTL_ENV_STR"
	os.Unsetenv(key)
	if got := envStr(key, "def"); got != "def" {
		t.Fatalf("envStr default: got %q", got)
	}
	os.Setenv(key, "val")
	t.Cleanup(func() { os.Unsetenv(key) })
	if got := envStr(key, "def"); got != "val" {
		t.Fatalf("envStr set: got %q", got)
	}
}

func TestEnvBool(t *testing.T) {
	key := "TESTCTL_ENV_BOOL"
	os.Unsetenv(key)
	if got := envBool(key, true); !got { t.Fatalf("envBool default true -> false") }
	if got := envBool(key, false); got { t.Fatalf("envBool default false -> true") }
	os.Setenv(key, "1"); t.Cleanup(func(){ os.Unsetenv(key) })
	if got := envBool(key, false); !got { t.Fatalf("envBool 1 -> false") }
	os.Setenv(key, "true")
	if got := envBool(key, false); !got { t.Fatalf("envBool true -> false") }
	os.Setenv(key, "yes")
	if got := envBool(key, false); !got { t.Fatalf("envBool yes -> false") }
	os.Setenv(key, "no")
	if got := envBool(key, true); got { t.Fatalf("envBool no -> true") }
}

func TestEnvInt(t *testing.T) {
	key := "TESTCTL_ENV_INT"
	os.Unsetenv(key)
	if got := envInt(key, 7); got != 7 { t.Fatalf("envInt default -> %d", got) }
	os.Setenv(key, "42"); t.Cleanup(func(){ os.Unsetenv(key) })
	if got := envInt(key, 0); got != 42 { t.Fatalf("envInt 42 -> %d", got) }
	os.Setenv(key, "bad")
	if got := envInt(key, 5); got != 5 { t.Fatalf("envInt bad -> %d", got) }
}

func TestMustNoopOnNil(t *testing.T) {
	// should not exit or panic if err is nil
	must(nil)
}
