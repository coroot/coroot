package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStackFunction(t *testing.T) {
	var function, colorBy string

	function, colorBy = parseStackFunction("/usr/local/lib/python3.12/threading.py Thread._bootstrap :0")
	assert.Equal(t, function, "Thread._bootstrap @/usr/local/lib/python3.12/threading.py")
	assert.Equal(t, "/usr/local/lib/python3.12/threading.py", colorBy)

	function, colorBy = parseStackFunction("/usr/local/lib/python3.12/site-packages/grpc/_channel.py _MultiThreadedRendezvous.__next__ :0")
	assert.Equal(t, function, "_MultiThreadedRendezvous.__next__ @grpc/_channel.py")
	assert.Equal(t, "grpc", colorBy)

	function, colorBy = parseStackFunction("boolean io.netty.channel.epoll.EpollEventLoop.processReady(io.netty.channel.epoll.EpollEventArray, int) :0")
	assert.Equal(t, "io.netty.channel.epoll.EpollEventLoop.processReady", function)
	assert.Equal(t, "io.netty.channel.epoll", colorBy)

	function, colorBy = parseStackFunction("void okhttp3.internal.connection.RealCall$AsyncCall.run() :0")
	assert.Equal(t, "okhttp3.internal.connection.RealCall$AsyncCall.run", function)
	assert.Equal(t, "okhttp3.internal.connection", colorBy)

	function, colorBy = parseStackFunction("github.com/prometheus/prometheus/scrape.(*scrapePool).sync.gowrap2 :0")
	assert.Equal(t, "github.com/prometheus/prometheus/scrape.(*scrapePool).sync.gowrap2", function)
	assert.Equal(t, "github.com/prometheus/prometheus", colorBy)
}
