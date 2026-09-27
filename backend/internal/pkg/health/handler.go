package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler is the HTTP adapter over Checker. It translates a Status into a
// status code and a JSON body — no probing logic lives here.
type Handler struct {
	checker *Checker
}

// NewHandler builds the health HTTP handler.
func NewHandler(checker *Checker) *Handler {
	return &Handler{checker: checker}
}

// RegisterRoutes mounts the health endpoints. They are intentionally
// unauthenticated: probes and load balancers must reach them.
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/healthz", h.Get)
}

// Get reports service health.
//
//	@Summary		Health check
//	@Description	Reports API and PostgreSQL connectivity. 200 when healthy, 503 when a dependency is down.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	Status
//	@Failure		503	{object}	Status
//	@Router			/healthz [get]
func (h *Handler) Get(ctx *gin.Context) {
	status := h.checker.Check(ctx.Request.Context())

	code := http.StatusOK
	if !status.Healthy() {
		code = http.StatusServiceUnavailable
	}

	ctx.JSON(code, status)
}
