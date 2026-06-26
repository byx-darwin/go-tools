package handler

import (
	"context"
	"time"

	"github.com/byx-darwin/go-tools/go-common/timeutil"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterTimeutilRoutes 注册 timeutil 示例路由。
func RegisterTimeutilRoutes(h *server.Hertz) {
	h.GET("/common/timeutil", timeutilHandler)
}

func timeutilHandler(_ context.Context, c *app.RequestContext) {
	now := time.Now().Unix()

	halfStart, halfEnd := timeutil.GetHalfYearMonth()
	allStart, allEnd := timeutil.GetAllYearMonth()
	monthStart, monthEnd := timeutil.MonthAdd(3)

	results := map[string]any{
		"format_yyyy_mm_dd": timeutil.Format(now, "YYYY-MM-DD", ""),
		"format_full":       timeutil.Format(now, "YYYY-MM-DD HH:mm:ss", ""),
		"format_utc":        timeutil.Format(now, "YYYY-MM-DD HH:mm:ss", "UTC"),
		"half_year":         map[string]any{"start": halfStart, "end": halfEnd},
		"all_year":          map[string]any{"start": allStart, "end": allEnd},
		"month_add_3":       map[string]any{"start": monthStart, "end": monthEnd},
	}

	hertzresp.Success(c, results)
}
