package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithHeaders_SetsHeadersOnConfig(t *testing.T) {
	t.Parallel()

	cfg := &GrainCallConfig{}
	WithHeaders(map[string]string{"trace": "abc", "tenant": "acme"})(cfg)

	assert.Len(t, cfg.Headers, 2)
	assert.Equal(t, "abc", cfg.Headers["trace"])
	assert.Equal(t, "acme", cfg.Headers["tenant"])
}

func TestWithHeaders_NilMap(t *testing.T) {
	t.Parallel()

	cfg := &GrainCallConfig{}
	WithHeaders(nil)(cfg)
	assert.Nil(t, cfg.Headers)
}
