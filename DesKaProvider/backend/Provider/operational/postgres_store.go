package operational

import (
    "database/sql"
    "errors"
    "fmt"
    "sort"
    "time"
)

type PostgresStore struct {
    db *sql.DB
}

func NewPostgresStore(db *sql.DB) (*PostgresStore, error) {
    if db == nil {
        return nil, errors.New("operational database is required")
    }
    return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Get(name string) (Snapshot, bool) {
    if name == "" {
        return Snapshot{}, false
    }
    var snapshot Snapshot
    var health string
    err := s.db.QueryRow(
        `SELECT provider_name, balance, currency, health, last_checked_at, last_success_at, last_error, consecutive_failures
         FROM provider_operational_snapshots WHERE provider_name = $1`,
        name,
    ).Scan(
        &snapshot.ProviderName,
        &snapshot.Balance,
        &snapshot.Currency,
        &health,
        &snapshot.LastCheckedAt,
        &snapshot.LastSuccessAt,
        &snapshot.LastError,
        &snapshot.ConsecutiveFailures,
    )
    if errors.Is(err, sql.ErrNoRows) {
        return Snapshot{}, false
    }
    if err != nil {
        return Snapshot{}, false
    }
    snapshot.Health = Health(health)
    return snapshot, true
}

func (s *PostgresStore) Put(snapshot Snapshot) error {
    if snapshot.ProviderName == "" {
        return errors.New("provider name is required")
    }
    if snapshot.Currency == "" {
        return errors.New("currency is required")
    }
    if snapshot.Health == "" {
        return errors.New("health is required")
    }
    _, err := s.db.Exec(
        `INSERT INTO provider_operational_snapshots
            (provider_name, balance, currency, health, last_checked_at, last_success_at, last_error, consecutive_failures)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
         ON CONFLICT (provider_name) DO UPDATE SET
            balance = EXCLUDED.balance,
            currency = EXCLUDED.currency,
            health = EXCLUDED.health,
            last_checked_at = EXCLUDED.last_checked_at,
            last_success_at = EXCLUDED.last_success_at,
            last_error = EXCLUDED.last_error,
            consecutive_failures = EXCLUDED.consecutive_failures`,
        snapshot.ProviderName,
        snapshot.Balance,
        snapshot.Currency,
        string(snapshot.Health),
        snapshot.LastCheckedAt,
        snapshot.LastSuccessAt,
        snapshot.LastError,
        snapshot.ConsecutiveFailures,
    )
    if err != nil {
        return fmt.Errorf("persist provider operational snapshot: %w", err)
    }
    return nil
}

func (s *PostgresStore) All() []Snapshot {
    rows, err := s.db.Query(
        `SELECT provider_name, balance, currency, health, last_checked_at, last_success_at, last_error, consecutive_failures
         FROM provider_operational_snapshots ORDER BY provider_name`,
    )
    if err != nil {
        return nil
    }
    defer rows.Close()

    result := make([]Snapshot, 0)
    for rows.Next() {
        var snapshot Snapshot
        var health string
        if err := rows.Scan(
            &snapshot.ProviderName,
            &snapshot.Balance,
            &snapshot.Currency,
            &health,
            &snapshot.LastCheckedAt,
            &snapshot.LastSuccessAt,
            &snapshot.LastError,
            &snapshot.ConsecutiveFailures,
        ); err != nil {
            return nil
        }
        snapshot.Health = Health(health)
        result = append(result, snapshot)
    }
    if err := rows.Err(); err != nil {
        return nil
    }
    sort.Slice(result, func(i, j int) bool { return result[i].ProviderName < result[j].ProviderName })
    return result
}

var _ Store = (*PostgresStore)(nil)

var _ = time.Time{}
