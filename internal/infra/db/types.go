package db

import (
	"database/sql"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/shared"
)

type NoteRow struct {
	ID        shared.ID
	Path      string
	Title     string
	Content   string
	Tags      []string
	Domains   []string
	Links     []LinkRow
	UpdatedAt string
}

type Query struct {
	Name string
	SQL  string
}

type NoteMeta struct {
	ID   int
	Path string
}

type NoteLiteRow struct {
	ID    int
	Title string
	Path  string
}

type NoteDegreeRow struct {
	ID    int
	Title string
	Path  string
	In    int
	Out   int
}

type LinkRow struct {
	Label        string
	Target       string
	Type         string
	ResolvedPath string
}

type TaskGroupRow struct {
	ID             int
	Name           string
	Level          int
	DerivedGroupID *int
	Status         string
	RawStatus      string
	NoteID         int
	LineNumber     int
}

type TodoRow struct {
	TaskGroupID int
	ID          int
	NoteID      int
	Task        string
	Status      string
	RawStatus   string
	Depth       int
	LineNumber  int
}

type TodoProjectionRow struct {
	ID            int
	NoteID        int
	NoteTitle     string
	Path          string
	TaskGroupID   int
	TaskGroupName string
	Task          string
	Status        string
	RawStatus     string
	Depth         int
	LineNumber    int
}

type RecipeRow struct {
	ID           int
	NoteID       int
	Name         string
	Path         string
	ServingSize  string
	PrepTime     string
	CookingTime  string
	Meal         string
	Instructions string
	PayloadJSON  string
	UpdatedAt    string
}

type IngredientRow struct {
	ID             int
	Name           string
	IngredientType string
	Notes          string
	RecipeCount    int
	MentionCount   int
	CreatedAt      string
	UpdatedAt      string
}

type RecipeIngredientMentionRow struct {
	ID                    int
	NoteID                int
	RecipeID              int
	RecipeName            string
	Path                  string
	RawText               string
	RawName               string
	QuantityText          string
	QuantityNumber        sql.NullFloat64
	UnitRaw               string
	CanonicalIngredientID sql.NullInt64
	CanonicalName         sql.NullString
	LineNumber            sql.NullInt64
}

type SyncOutboxRow struct {
	ID             int64
	OpID           string
	IdempotencyKey string
	EntityType     string
	EntityKey      string
	OpType         string
	Payload        string
	PayloadHash    string
	BaseVersion    int
	Status         string
	AttemptCount   int
	LastError      string
	CreatedAt      string
	UpdatedAt      string
	AckedAt        sql.NullString
}

type SyncConflictRow struct {
	ID            int64
	EntityType    string
	EntityKey     string
	LocalPayload  string
	RemotePayload string
	Reason        string
	CreatedAt     string
	ResolvedAt    sql.NullString
}

type SyncConflictFilter struct {
	Limit          int
	UnresolvedOnly bool
	CreatedBefore  *time.Time
}

type SyncTodoRow struct {
	ID          string
	SourceID    string
	SourcePath  sql.NullString
	TaskScope   sql.NullString
	TaskArea    sql.NullString
	TodoSection string
	TaskText    string
	IsDone      bool
	Meta        sql.NullString
	TaskOrder   int
	LineNumber  sql.NullInt64
	UpdatedAt   string
	Payload     string
}

type SyncDiagnostics struct {
	PendingOutboxCount           int
	PendingOutboxRetriedCount    int
	PendingOutboxMaxAttemptCount int
	PendingOutboxOldestCreatedAt string
	PendingOutboxLatestFailure   string
	AckedOutboxCount             int
	TotalConflictCount           int
	UnresolvedConflictCount      int
	OldestUnresolvedConflictAt   string
	LocalCursor                  string
	SyncEntityVersionCount       int
	SyncedTodoCount              int
}
