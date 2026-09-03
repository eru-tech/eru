package chunking

import (
	"os"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("eru-ai", "test")
	os.Exit(m.Run())
}
