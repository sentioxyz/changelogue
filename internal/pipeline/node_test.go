// internal/pipeline/node_test.go
package pipeline

import (
	"errors"
	"testing"
)

func TestErrEventDropped(t *testing.T) {
	if !errors.Is(ErrEventDropped, ErrEventDropped) {
		t.Error("ErrEventDropped should be identifiable with errors.Is")
	}
}
