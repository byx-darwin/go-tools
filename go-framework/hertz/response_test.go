package hertz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponse_Struct(t *testing.T) {
	r := Response{Code: 200, Msg: "ok", Data: map[string]string{"id": "123"}}
	assert.Equal(t, 200, r.Code)
	assert.Equal(t, "ok", r.Msg)
	assert.NotNil(t, r.Data)
}

func TestResponse_NoData(t *testing.T) {
	r := Response{Code: 500, Msg: "error"}
	assert.Nil(t, r.Data)
}

func TestResponse_JSONTags(t *testing.T) {
	r := Response{Code: 200, Msg: "success"}
	assert.Equal(t, 200, r.Code)
}
