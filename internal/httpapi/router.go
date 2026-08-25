package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/11DingKing/lushan-study-ops-go/internal/service"
)

type API struct {
	service *service.Service
	logger  *slog.Logger
}

func New(svc *service.Service, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	api := &API{service: svc, logger: logger}
	public := http.NewServeMux()
	public.HandleFunc("GET /healthz", api.health)
	public.HandleFunc("GET /readyz", api.ready)
	public.HandleFunc("POST /v1/auth/login", api.login)

	private := http.NewServeMux()
	private.HandleFunc("POST /v1/auth/logout", api.logout)
	private.HandleFunc("POST /v1/applications", api.apply)
	private.HandleFunc("POST /v1/cohorts/{id}/decision", api.decide)
	private.HandleFunc("GET /v1/cohorts", api.listCohorts)
	private.HandleFunc("POST /v1/cohorts/{id}/plan-items", api.addPlanItem)
	private.HandleFunc("POST /v1/cohorts/{id}/acknowledgements", api.acknowledge)
	private.HandleFunc("POST /v1/cohorts/{id}/confirm", api.confirm)
	private.HandleFunc("POST /v1/cohorts/{id}/attendance-groups", api.createAttendanceGroup)
	private.HandleFunc("POST /v1/cohorts/{id}/attendance", api.recordAttendance)
	private.HandleFunc("POST /v1/cohorts/{id}/attendance/batch", api.recordAttendanceBatch)
	private.HandleFunc("POST /v1/cohorts/{id}/reroutes", api.reroute)
	private.HandleFunc("POST /v1/cohorts/{id}/artifacts", api.archiveArtifact)
	private.HandleFunc("POST /v1/cohorts/{id}/cancel", api.cancel)
	public.Handle("/v1/", authenticate(svc)(private))

	return chain(public, requestID, recoverPanic(logger), accessLog(logger))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	if err := a.service.Repository().Ping(r.Context()); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
