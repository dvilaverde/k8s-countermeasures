package assets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetRestartPatch(t *testing.T) {
	patch := GetPatch("restart-patch.yaml")
	assert.NotNil(t, patch)
	assert.Equal(t, types.MergePatchType, patch.Type())
}

func TestInvalidPatch(t *testing.T) {
	assert.Panics(t, func() {
		patch := GetPatch("no-patch.yaml")
		assert.NotNil(t, patch)
		assert.Equal(t, types.MergePatchType, patch.Type())
	}, "the missing resource should have caused a panic")
}
