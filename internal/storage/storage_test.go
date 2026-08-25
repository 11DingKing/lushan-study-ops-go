package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenMemory(context.Background(), t.Name())
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func testUser(id, email string, role domain.Role, now time.Time) domain.User {
	return domain.User{ID: id, Email: email, Name: id, Role: role, PasswordHash: "hash",
		Active: true, CreatedAt: now}
}

func insertUsers(t *testing.T, store *Store, users ...domain.User) {
	t.Helper()
	for _, user := range users {
		if err := store.CreateUser(context.Background(), user); err != nil {
			t.Fatalf("CreateUser(%s) error = %v", user.ID, err)
		}
	}
}

func testCohort(ownerID string, now time.Time) (domain.Cohort, domain.Application) {
	cohort := domain.Cohort{ID: "coh-1", OwnerUserID: ownerID, Name: "Lushan field studies", Kind: "school",
		ParticipantCount: 30, Status: domain.CohortApplied, PlanRevision: 1, Version: 1,
		StartsAt: now.Add(24 * time.Hour), EndsAt: now.Add(32 * time.Hour), CreatedAt: now, UpdatedAt: now}
	application := domain.Application{ID: "app-1", CohortID: cohort.ID, School: "Example School",
		Contact: "Leader", Status: domain.ApplicationSubmitted, CreatedAt: now, UpdatedAt: now}
	return cohort, application
}

func TestMigrationsBuildCompleteRelationalSchema(t *testing.T) {
	store := testStore(t)
	var migrationCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 1 {
		t.Fatalf("migration count = %d", migrationCount)
	}
	var tableCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount < 20 {
		t.Fatalf("table count = %d, want at least 20", tableCount)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
	var after int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != migrationCount {
		t.Fatalf("repeat migration count = %d", after)
	}
}

func TestForeignKeysRejectDanglingBusinessRows(t *testing.T) {
	store := testStore(t)
	_, err := store.db.Exec(`INSERT INTO applications
        (id, cohort_id, school, contact, status, notes, created_at, updated_at)
        VALUES ('app', 'missing', 'school', 'contact', 'submitted', '', ?, ?)`,
		formatTime(time.Now()), formatTime(time.Now()))
	if err == nil {
		t.Fatal("dangling application insert succeeded")
	}
}

func TestUserAndSessionLifecyclePersists(t *testing.T) {
	store := testStore(t)
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	user := testUser("usr-1", "Leader@Example.Test", domain.RoleLeader, now)
	if err := store.CreateUser(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	byEmail, err := store.FindUserByEmail(context.Background(), "leader@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if byEmail.ID != user.ID || byEmail.Role != domain.RoleLeader || !byEmail.Active {
		t.Fatalf("persisted user = %+v", byEmail)
	}
	session := domain.Session{ID: "ses-1", UserID: user.ID, TokenHash: "token-hash",
		ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	if err := store.CreateSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	gotSession, gotUser, err := store.FindSessionByTokenHash(context.Background(), "token-hash")
	if err != nil {
		t.Fatal(err)
	}
	if gotSession.ID != session.ID || gotUser.ID != user.ID || !gotSession.ActiveAt(now) {
		t.Fatalf("session/user = %+v / %+v", gotSession, gotUser)
	}
	if err := store.RevokeSession(context.Background(), session.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	gotSession, _, err = store.FindSessionByTokenHash(context.Background(), "token-hash")
	if err != nil {
		t.Fatal(err)
	}
	if gotSession.RevokedAt == nil || gotSession.ActiveAt(now.Add(2*time.Minute)) {
		t.Fatalf("revoked session = %+v", gotSession)
	}
}

func TestDuplicateUserEmailReturnsConflict(t *testing.T) {
	store := testStore(t)
	now := time.Now().UTC()
	insertUsers(t, store, testUser("usr-1", "same@example.test", domain.RoleLeader, now))
	err := store.CreateUser(context.Background(), testUser("usr-2", "SAME@example.test", domain.RoleOperator, now))
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestTransactionRollsBackEveryWrite(t *testing.T) {
	store := testStore(t)
	now := time.Now().UTC()
	intentional := errors.New("audit unavailable")
	err := store.Transact(context.Background(), func(ctx context.Context, repo repository.Repository) error {
		if err := repo.CreateUser(ctx, testUser("usr-rollback", "rollback@example.test", domain.RoleOperator, now)); err != nil {
			return err
		}
		return intentional
	})
	if !errors.Is(err, intentional) {
		t.Fatalf("transaction error = %v", err)
	}
	_, err = store.FindUserByID(context.Background(), "usr-rollback")
	if !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("rolled back user lookup = %v", err)
	}
}

func TestNestedRepositoryTransactionUsesOuterAtomicBoundary(t *testing.T) {
	store := testStore(t)
	now := time.Now().UTC()
	insertUsers(t, store, testUser("leader", "leader@example.test", domain.RoleLeader, now))
	cohort, application := testCohort("leader", now)
	err := store.Transact(context.Background(), func(ctx context.Context, repo repository.Repository) error {
		if err := repo.CreateCohort(ctx, cohort, application); err != nil {
			return err
		}
		return errors.New("cancel outer unit")
	})
	if err == nil {
		t.Fatal("outer transaction succeeded")
	}
	if _, err := store.GetCohort(context.Background(), cohort.ID); !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("cohort survived rollback: %v", err)
	}
	if _, err := store.GetApplicationByCohort(context.Background(), cohort.ID); !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("application survived rollback: %v", err)
	}
}

func TestDatabaseStateSurvivesCloseAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	ctx := context.Background()
	first, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := first.CreateUser(ctx, testUser("usr-restart", "restart@example.test", domain.RoleSafety, now)); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	user, err := second.FindUserByID(ctx, "usr-restart")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "restart@example.test" || user.Role != domain.RoleSafety {
		t.Fatalf("recovered user = %+v", user)
	}
}

func TestCohortListFiltersAndPaginates(t *testing.T) {
	store := testStore(t)
	now := time.Now().UTC()
	insertUsers(t, store, testUser("leader", "leader@example.test", domain.RoleLeader, now))
	for index := 0; index < 5; index++ {
		cohort, application := testCohort("leader", now.Add(time.Duration(index)*time.Hour))
		cohort.ID = "coh-" + string(rune('a'+index))
		application.ID = "app-" + string(rune('a'+index))
		application.CohortID = cohort.ID
		if err := store.CreateCohort(context.Background(), cohort, application); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := store.ListCohorts(context.Background(), repository.CohortFilter{
		OwnerID: "leader", Status: domain.CohortApplied, Page: repository.Page{Limit: 2, Offset: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(items) != 2 {
		t.Fatalf("total/items = %d/%d", total, len(items))
	}
	if !items[0].StartsAt.Before(items[1].StartsAt) {
		t.Fatalf("items are not ordered: %+v", items)
	}
}

func TestConcurrentOptimisticStatusUpdatesHaveOneWinner(t *testing.T) {
	store := testStore(t)
	now := time.Now().UTC()
	insertUsers(t, store, testUser("leader", "leader@example.test", domain.RoleLeader, now))
	cohort, application := testCohort("leader", now)
	if err := store.CreateCohort(context.Background(), cohort, application); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, status := range []domain.CohortStatus{domain.CohortPlanned, domain.CohortCancelled} {
		status := status
		go func() {
			<-start
			results <- store.UpdateCohortStatus(context.Background(), cohort.ID, cohort.Version, status, now.Add(time.Minute))
		}()
	}
	close(start)
	first := <-results
	second := <-results
	successes := 0
	conflicts := 0
	for _, err := range []error{first, second} {
		if err == nil {
			successes++
		} else if apperr.IsCode(err, apperr.CodeConflict) || apperr.IsCode(err, apperr.CodeUnavailable) {
			conflicts++
		} else {
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success/conflict = %d/%d, errors = %v/%v", successes, conflicts, first, second)
	}
	updated, err := store.GetCohort(context.Background(), cohort.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != cohort.Version+1 {
		t.Fatalf("updated version = %d", updated.Version)
	}
}

func TestCanceledContextStopsRepositoryQuery(t *testing.T) {
	store := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := store.ListCohorts(ctx, repository.CohortFilter{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListCohorts(canceled) error = %v", err)
	}
}

func TestCreateAttendanceGroupStopsOnCanceledContext(t *testing.T) {
	fixture := setupResources(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	group := domain.AttendanceGroup{ID: "grp-canceled", CohortID: fixture.cohort.ID, Name: "Canceled route",
		MentorID: fixture.mentorID, Capacity: 2, Version: 1}
	err := fixture.store.CreateAttendanceGroup(ctx, group)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateAttendanceGroup(canceled) error = %v", err)
	}
	var count int
	if err := fixture.store.db.QueryRow(`SELECT COUNT(*) FROM attendance_groups WHERE cohort_id = ?`,
		fixture.cohort.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("canceled request persisted %d attendance groups", count)
	}
}
