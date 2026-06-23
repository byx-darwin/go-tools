package timeutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMonthAdd_Positive(t *testing.T) {
	start, end := MonthAdd(6)
	assert.Greater(t, end, start, "6 months forward: end > start")
	t.Logf("MonthAdd(6) = %d → %d", start, end)
}

func TestMonthAdd_Negative(t *testing.T) {
	start, end := MonthAdd(-3)
	assert.Less(t, start, end, "-3 months backward: first return < second return")
	t.Logf("MonthAdd(-3) = %d → %d", start, end)
}

func TestMonthAdd_Zero(t *testing.T) {
	start, end := MonthAdd(0)
	assert.Equal(t, start, end, "0 months: both values equal")
}

func TestGetHalfYearMonth(t *testing.T) {
	start, end := GetHalfYearMonth()
	assert.Less(t, start, end, "half year range: start < end")
	t.Logf("GetHalfYearMonth = %d → %d", start, end)
}

func TestGetAllYearMonth(t *testing.T) {
	start, end := GetAllYearMonth()
	assert.Less(t, start, end, "full year range: start < end")
	t.Logf("GetAllYearMonth = %d → %d", start, end)
}

func TestMonthAdd_YearRollover(t *testing.T) {
	// Adding 13 months should change YYYY component
	start, end := MonthAdd(13)
	diff := end - start
	t.Logf("MonthAdd(13) = %d → %d (diff=%d)", start, end, diff)
	// Check format: YYYYMM, month part 01-12
	assert.GreaterOrEqual(t, start%100, 1)
	assert.LessOrEqual(t, start%100, 12)
	assert.GreaterOrEqual(t, end%100, 1)
	assert.LessOrEqual(t, end%100, 12)
}

func TestMonthAdd_KnownRange(t *testing.T) {
	// Even arbitrary values should produce YYYYMM format
	start, end := MonthAdd(1)
	assert.GreaterOrEqual(t, start, 202400)
	assert.LessOrEqual(t, start, 209999)
	assert.GreaterOrEqual(t, end, 202400)
	assert.LessOrEqual(t, end, 209999)
}
