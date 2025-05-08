package time

import "testing"

func TestMonthAdd(t *testing.T) {
	start, end := MonthAdd(6)
	t.Log(start, end)
}

func TestGetHalfYearMonth(t *testing.T) {
	start, end := GetHalfYearMonth()
	t.Log(start, end)
}

func TestGetAllYearMonth(t *testing.T) {
	start, end := GetAllYearMonth()
	t.Log(start, end)
}
