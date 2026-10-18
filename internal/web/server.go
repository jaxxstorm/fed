package web

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// Bootstrap serves operational endpoints. Protected application routes are
// registered separately once their identity and domain services are available.
func Bootstrap(ping func(context.Context) error) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(requestLogging)
	e.GET("/healthz", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	e.GET("/readyz", func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		return c.NoContent(http.StatusOK)
	})
	return e
}
