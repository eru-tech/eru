package cache

import (
	"os"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("eru-cache-test", "test-instance")
	os.Exit(m.Run())
}
