package utils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeJsonMultilineStrings(t *testing.T) {
	src := []byte(`
foo: bar
buz
`)
	assert.False(t, json.Valid(src))
	assert.Equal(t, src, EscapeJsonMultilineStrings(src))

	src = []byte(`""`)
	assert.True(t, json.Valid(src))
	assert.Equal(t, src, EscapeJsonMultilineStrings(src))

	src = []byte(`
{"foo": "
"}
`)
	assert.False(t, json.Valid(src))
	assert.True(t, json.Valid(EscapeJsonMultilineStrings(src)))
	expected := []byte(`
{"foo": "\n"}
`)
	assert.Equal(t, expected, EscapeJsonMultilineStrings(src))

	src = []byte(`
{"foo": "bar
buz"}
`)
	assert.False(t, json.Valid(src))
	assert.True(t, json.Valid(EscapeJsonMultilineStrings(src)))
	expected = []byte(`
{"foo": "bar\nbuz"}
`)
	assert.Equal(t, expected, EscapeJsonMultilineStrings(src))

	src = []byte(`{
 "text": "foo
bar",
  "param": "buz
boom" 
}
`)
	assert.False(t, json.Valid(src))
	assert.True(t, json.Valid(EscapeJsonMultilineStrings(src)))
	expected = []byte(`{
 "text": "foo\nbar",
  "param": "buz\nboom" 
}
`)
	assert.Equal(t, expected, EscapeJsonMultilineStrings(src))

	src = []byte(`{
 "text": "Hello, 世界
Hello, 
世界",
  "param": "buz
boom" 
}
`)
	assert.False(t, json.Valid(src))
	assert.True(t, json.Valid(EscapeJsonMultilineStrings(src)))
	expected = []byte(`{
 "text": "Hello, 世界\nHello, \n世界",
  "param": "buz\nboom" 
}
`)
	assert.Equal(t, expected, EscapeJsonMultilineStrings(src))
}
