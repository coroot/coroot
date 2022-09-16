package chunk

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCompression(t *testing.T) {
	data := []byte("xzxzxzxzxf2w0er-kwedew-d0kwed0-")
	compressed, err := compress(data)
	require.NoError(t, err)
	decompressed, err := decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)

}
