package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/clock"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/service"
	"github.com/11DingKing/lushan-study-ops-go/internal/storage"
)

type apiFixture struct {
	handler http.Handler
	service *service.Service
	store   *storage.Store
	clock   *clock.Fake
}

func setupAPI(t *testing.T) apiFixture {
	t.Helper()
	store, err := storage.OpenMemory(context.Background(), t.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	clk := clock.NewFake(time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC))
	svc := service.New(store, clk, time.Hour, 3)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return apiFixture{handler: New(svc, logger), service: svc, store: store, clock: clk}
}

func createAPIUser(t *testing.T, fixture apiFixture, email, password string, role domain.Role) domain.User {
	t.Helper()
	user, err := fixture.service.CreateUser(context.Background(), email, string(role), password, role)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func perform(handler http.Handler, method, path, token string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func loginAPI(t *testing.T, fixture apiFixture, email, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	response := perform(fixture.handler, http.MethodPost, "/v1/auth/login", "", string(body))
	if response.Code != http.StatusOK {
		t.Fatalf("login status/body = %d/%s", response.Code, response.Body.String())
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Token == "" {
		t.Fatal("login returned empty token")
	}
	return result.Token
}

func TestHealthAndReadinessExposeDependencyState(t *testing.T) {
	fixture := setupAPI(t)
	health := perform(fixture.handler, http.MethodGet, "/healthz", "", "")
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"status":"ok"`) {
		t.Fatalf("health = %d %s", health.Code, health.Body.String())
	}
	ready := perform(fixture.handler, http.MethodGet, "/readyz", "", "")
	if ready.Code != http.StatusOK || !strings.Contains(ready.Body.String(), `"status":"ready"`) {
		t.Fatalf("ready = %d %s", ready.Code, ready.Body.String())
	}
	if ready.Header().Get("X-Request-ID") == "" {
		t.Fatal("readiness response has no request ID")
	}
}

func TestLoginProtectedListAndLogoutRevocation(t *testing.T) {
	fixture := setupAPI(t)
	createAPIUser(t, fixture, "operator@example.test", "operator-password", domain.RoleOperator)
	token := loginAPI(t, fixture, "operator@example.test", "operator-password")
	listed := perform(fixture.handler, http.MethodGet, "/v1/cohorts?limit=10&offset=0", token, "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list status/body = %d/%s", listed.Code, listed.Body.String())
	}
	if !strings.Contains(listed.Body.String(), `"items":[]`) || !strings.Contains(listed.Body.String(), `"total":0`) {
		t.Fatalf("list body = %s", listed.Body.String())
	}
	logout := perform(fixture.handler, http.MethodPost, "/v1/auth/logout", token, "")
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status/body = %d/%s", logout.Code, logout.Body.String())
	}
	denied := perform(fixture.handler, http.MethodGet, "/v1/cohorts", token, "")
	if denied.Code != http.StatusUnauthorized || !strings.Contains(denied.Body.String(), `"code":"expired"`) {
		t.Fatalf("revoked request = %d %s", denied.Code, denied.Body.String())
	}
}

func TestProtectedRoutesRequireBearerScheme(t *testing.T) {
	fixture := setupAPI(t)
	for _, header := range []string{"", "token", "Basic abc", "Bearer"} {
		request := httptest.NewRequest(http.MethodGet, "/v1/cohorts", nil)
		request.Header.Set("Authorization", header)
		response := httptest.NewRecorder()
		fixture.handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("header %q status = %d", header, response.Code)
		}
		if !strings.Contains(response.Body.String(), `"request_id":`) {
			t.Fatalf("header %q body lacks request id: %s", header, response.Body.String())
		}
	}
}

func TestApplicationEndpointPersistsPublicWorkflow(t *testing.T) {
	fixture := setupAPI(t)
	createAPIUser(t, fixture, "leader@example.test", "leader-password", domain.RoleLeader)
	token := loginAPI(t, fixture, "leader@example.test", "leader-password")
	payload := map[string]any{
		"name":              "Autumn geology class",
		"kind":              "school",
		"participant_count": 36,
		"school":            "Example School",
		"contact":           "Ms. Li",
		"notes":             "middle school",
		"starts_at":         fixture.clock.Now().Add(24 * time.Hour),
		"ends_at":           fixture.clock.Now().Add(30 * time.Hour),
	}
	body, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/v1/applications", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "school-request-42")
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("application status/body = %d/%s", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Request-ID") != "school-request-42" {
		t.Fatalf("request ID = %q", response.Header().Get("X-Request-ID"))
	}
	var cohort domain.Cohort
	if err := json.Unmarshal(response.Body.Bytes(), &cohort); err != nil {
		t.Fatal(err)
	}
	if cohort.Status != domain.CohortApplied || cohort.ParticipantCount != 36 || cohort.OwnerUserID == "" {
		t.Fatalf("created cohort = %+v", cohort)
	}
	persisted, err := fixture.store.GetCohort(context.Background(), cohort.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Name != "Autumn geology class" || persisted.Version != 1 {
		t.Fatalf("persisted cohort = %+v", persisted)
	}
}

func TestMalformedAndUnknownJSONFieldsMapToStableError(t *testing.T) {
	fixture := setupAPI(t)
	createAPIUser(t, fixture, "leader@example.test", "leader-password", domain.RoleLeader)
	token := loginAPI(t, fixture, "leader@example.test", "leader-password")
	for _, body := range []string{
		`{"name":`,
		`{"name":"group","unknown":true}`,
		`{} {}`,
	} {
		response := perform(fixture.handler, http.MethodPost, "/v1/applications", token, body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d: %s", body, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `"code":"invalid_argument"`) {
			t.Fatalf("body %q response = %s", body, response.Body.String())
		}
	}
}

func TestRoleFailureMapsToForbidden(t *testing.T) {
	fixture := setupAPI(t)
	createAPIUser(t, fixture, "mentor@example.test", "mentor-password", domain.RoleMentor)
	token := loginAPI(t, fixture, "mentor@example.test", "mentor-password")
	payload := `{"name":"group","kind":"school","participant_count":10,"school":"x","contact":"y","starts_at":"2026-09-01T08:00:00Z","ends_at":"2026-09-01T10:00:00Z"}`
	response := perform(fixture.handler, http.MethodPost, "/v1/applications", token, payload)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"forbidden"`) {
		t.Fatalf("forbidden response = %d %s", response.Code, response.Body.String())
	}
}

func TestPaginationValidationRejectsNegativeAndMalformedValues(t *testing.T) {
	fixture := setupAPI(t)
	createAPIUser(t, fixture, "operator@example.test", "operator-password", domain.RoleOperator)
	token := loginAPI(t, fixture, "operator@example.test", "operator-password")
	for _, path := range []string{
		"/v1/cohorts?limit=-1",
		"/v1/cohorts?offset=-5",
		"/v1/cohorts?limit=many",
	} {
		response := perform(fixture.handler, http.MethodGet, path, token, "")
		if response.Code != http.StatusBadRequest {
			t.Fatalf("path %s status/body = %d/%s", path, response.Code, response.Body.String())
		}
	}
}
