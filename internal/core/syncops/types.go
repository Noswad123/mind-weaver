package syncops

type EntityType string

const (
	EntityNote EntityType = "note"
	EntityTodo EntityType = "todo"
)

type OperationType string

const (
	OperationUpsert OperationType = "upsert"
	OperationDelete OperationType = "delete"
)

type OperationStatus string

const (
	StatusPending OperationStatus = "pending"
	StatusAcked   OperationStatus = "acked"
	StatusFailed  OperationStatus = "failed"
)
