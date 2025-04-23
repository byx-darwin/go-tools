package time

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestTimeFormat(t *testing.T) {
	sec := time.Now().Unix()
	tz := "America/Cordoba"
	actual := Format(sec, "YYYYMMDD HH:mm:ss", tz)
	l, _ := time.LoadLocation(tz)
	expected := time.Unix(sec, 0).In(l).Format("2006-01-02 15:04:05")
	assert.Equal(t, expected, actual)
}
