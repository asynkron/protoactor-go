package natskv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_ReturnsProvider(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNewFromJetStream_ReturnsProvider(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	p, err := NewFromJetStream(js)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNew_WithOptions(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc,
		WithBucketName("custom_bucket"),
		WithKeyPrefix("myprefix"),
	)
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "custom_bucket", p.config.BucketName)
	assert.Equal(t, "myprefix", p.config.KeyPrefix)
}

func TestProvider_GetHealthStatus_NilByDefault(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	assert.NoError(t, p.GetHealthStatus())
}
