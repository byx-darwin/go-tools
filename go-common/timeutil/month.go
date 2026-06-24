package timeutil

import (
	"fmt"
	"strconv"
	"time"
)

// MonthAdd 计算当前时间加减 months 个月后的起止月份。
// 返回 (startYearMonth, endYearMonth) 格式为 YYYYMM 整数，按时间升序排列。
func MonthAdd(months int) (int, int) {
	cUnix := time.Now()
	cYear := cUnix.Year()
	cMonth := cUnix.Month()
	cMonthStr := fmt.Sprintf("%d%02d", cYear, cMonth)

	tUnix := cUnix.AddDate(0, months, 0)
	tYear := tUnix.Year()
	tMonth := tUnix.Month()
	tMonthStr := fmt.Sprintf("%d%02d", tYear, tMonth)
	startMonth, _ := strconv.Atoi(cMonthStr)
	endMonth, _ := strconv.Atoi(tMonthStr)
	if months < 0 {
		return endMonth, startMonth
	} else {
		return startMonth, endMonth
	}
}

// GetHalfYearMonth 获取最近 6 个月的起止月份（YYYYMM 格式）。
func GetHalfYearMonth() (int, int) {
	return MonthAdd(-5)
}

// GetAllYearMonth 获取最近 12 个月的起止月份（YYYYMM 格式）。
func GetAllYearMonth() (int, int) {
	return MonthAdd(-11)
}
