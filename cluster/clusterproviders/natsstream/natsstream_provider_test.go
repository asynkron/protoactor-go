package natsstream

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
		WithStreamName("MY_STREAM"),
		WithSubjectPrefix("myprefix"),
	)
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "MY_STREAM", p.config.StreamName)
	assert.Equal(t, "myprefix", p.config.SubjectPrefix)
}

func TestProvider_GetHealthStatus_NilByDefault(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	assert.NoError(t, p.GetHealthStatus())
}

func TestProvider_IdentityLookup_NotNil(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	assert.NotNil(t, p.IdentityLookup())
}
