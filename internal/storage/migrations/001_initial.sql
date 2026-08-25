PRAGMA foreign_keys = ON;

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('leader','operator','venue_admin','mentor','safety')),
    password_hash TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)),
    created_at TEXT NOT NULL
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_user_active ON sessions(user_id, expires_at, revoked_at);

CREATE TABLE cohorts (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('school','family')),
    participant_count INTEGER NOT NULL CHECK (participant_count BETWEEN 1 AND 500),
    status TEXT NOT NULL,
    plan_revision INTEGER NOT NULL DEFAULT 1,
    version INTEGER NOT NULL DEFAULT 1,
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_cohorts_owner_status ON cohorts(owner_user_id, status, starts_at);

CREATE TABLE applications (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL UNIQUE REFERENCES cohorts(id) ON DELETE CASCADE,
    school TEXT NOT NULL,
    contact TEXT NOT NULL,
    status TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE course_units (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    venue_type TEXT NOT NULL,
    duration_min INTEGER NOT NULL CHECK (duration_min > 0),
    risk_level TEXT NOT NULL CHECK (risk_level IN ('low','medium','high')),
    min_age INTEGER NOT NULL DEFAULT 0,
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1))
);

CREATE TABLE venues (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL,
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1))
);

CREATE TABLE mentors (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
    specialty TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1))
);

CREATE TABLE plan_items (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL REFERENCES cohorts(id) ON DELETE CASCADE,
    course_unit_id TEXT NOT NULL REFERENCES course_units(id),
    venue_id TEXT NOT NULL REFERENCES venues(id),
    mentor_id TEXT NOT NULL REFERENCES mentors(id),
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL,
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    revision INTEGER NOT NULL,
    UNIQUE(cohort_id, revision, starts_at)
);
CREATE INDEX idx_plan_items_cohort_revision ON plan_items(cohort_id, revision, starts_at);

CREATE TABLE venue_holds (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL REFERENCES cohorts(id) ON DELETE CASCADE,
    plan_item_id TEXT NOT NULL UNIQUE REFERENCES plan_items(id) ON DELETE CASCADE,
    venue_id TEXT NOT NULL REFERENCES venues(id),
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL,
    seats INTEGER NOT NULL CHECK (seats > 0),
    status TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_venue_holds_conflict ON venue_holds(venue_id, starts_at, ends_at, status);

CREATE TABLE mentor_assignments (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL REFERENCES cohorts(id) ON DELETE CASCADE,
    plan_item_id TEXT NOT NULL UNIQUE REFERENCES plan_items(id) ON DELETE CASCADE,
    mentor_id TEXT NOT NULL REFERENCES mentors(id),
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL,
    status TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_mentor_assignments_conflict ON mentor_assignments(mentor_id, starts_at, ends_at, status);

CREATE TABLE risk_acknowledgements (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL REFERENCES cohorts(id) ON DELETE CASCADE,
    actor_user_id TEXT NOT NULL REFERENCES users(id),
    subject_type TEXT NOT NULL,
    subject_ref TEXT NOT NULL,
    plan_revision INTEGER NOT NULL,
    acknowledged_at TEXT NOT NULL,
    UNIQUE(cohort_id, actor_user_id, subject_type, subject_ref, plan_revision)
);
CREATE INDEX idx_ack_cohort_revision ON risk_acknowledgements(cohort_id, plan_revision);

CREATE TABLE attendance_groups (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL REFERENCES cohorts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    mentor_id TEXT NOT NULL REFERENCES mentors(id),
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    version INTEGER NOT NULL DEFAULT 1,
    UNIQUE(cohort_id, name)
);

CREATE TABLE attendance_records (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL REFERENCES cohorts(id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES attendance_groups(id),
    participant_ref TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('present','late','absent')),
    checked_in_at TEXT,
    recorded_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL,
    UNIQUE(cohort_id, participant_ref)
);
CREATE INDEX idx_attendance_group_status ON attendance_records(group_id, status);

CREATE TABLE weather_alerts (
    id TEXT PRIMARY KEY,
    venue_id TEXT NOT NULL REFERENCES venues(id),
    severity TEXT NOT NULL,
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_weather_active ON weather_alerts(venue_id, starts_at, ends_at, status);

CREATE TABLE reroutes (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL REFERENCES cohorts(id),
    plan_item_id TEXT NOT NULL REFERENCES plan_items(id),
    from_venue_id TEXT NOT NULL REFERENCES venues(id),
    to_venue_id TEXT NOT NULL REFERENCES venues(id),
    reason TEXT NOT NULL,
    requested_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL
);

CREATE TABLE artifacts (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL REFERENCES cohorts(id),
    participant_ref TEXT NOT NULL,
    kind TEXT NOT NULL,
    uri TEXT NOT NULL,
    checksum TEXT NOT NULL,
    archived_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL,
    UNIQUE(cohort_id, participant_ref, kind, checksum)
);

CREATE TABLE settlements (
    id TEXT PRIMARY KEY,
    cohort_id TEXT NOT NULL UNIQUE REFERENCES cohorts(id),
    gross_cents INTEGER NOT NULL CHECK (gross_cents >= 0),
    refund_cents INTEGER NOT NULL CHECK (refund_cents >= 0),
    fee_cents INTEGER NOT NULL CHECK (fee_cents >= 0),
    currency TEXT NOT NULL,
    status TEXT NOT NULL,
    policy_code TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE outbox_jobs (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    payload BLOB NOT NULL,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL,
    available_at TEXT NOT NULL,
    locked_at TEXT,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_outbox_claim ON outbox_jobs(status, available_at, created_at);

CREATE TABLE idempotency_keys (
    scope TEXT NOT NULL,
    key TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    response BLOB NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    PRIMARY KEY(scope, key)
);
CREATE INDEX idx_idempotency_expiry ON idempotency_keys(expires_at);

CREATE TABLE audit_events (
    id TEXT PRIMARY KEY,
    actor_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    action TEXT NOT NULL,
    object_type TEXT NOT NULL,
    object_id TEXT NOT NULL,
    result TEXT NOT NULL,
    detail TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_audit_object ON audit_events(object_type, object_id, created_at);
CREATE INDEX idx_audit_request ON audit_events(request_id);
