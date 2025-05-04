package errors

type ErrMissingCatalog struct {
	*Error
	Errs []error
	Msgs []any
}

type ErrMissingProvider struct {
	*Error
	Errs []error
	Msgs []any
}

type ErrMissingTaskStore struct {
	*Error
	Errs []error
	Msgs []any
}

type ErrMissingTaskManager struct {
	*Error
	Errs []error
	Msgs []any
}
