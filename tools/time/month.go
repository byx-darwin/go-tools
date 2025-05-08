package time

import (
	"fmt"
	"strconv"
	"time"
)

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

func GetHalfYearMonth() (int, int) {
	return MonthAdd(-5)
}

func GetAllYearMonth() (int, int) {
	return MonthAdd(-11)
}
