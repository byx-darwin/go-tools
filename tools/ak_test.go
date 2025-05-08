package tools

import "testing"

func TestGetRandAk(t *testing.T) {
	ak := GetRandAk(10)
	t.Log(ak)
}

func TestRefreshSK(t *testing.T) {
	ak := GetRandAk(10)
	t.Log(ak)
	t.Log(RefreshSK(ak))
}
