package pluginbinding

type ListResult[T any] struct {
	Items []T `json:"items"`
	Count int `json:"count"`
}

type ShowResult[T any] struct {
	Record   T              `json:"record"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func NewListResult[T any](items []T) ListResult[T] {
	return ListResult[T]{Items: items, Count: len(items)}
}

func NewShowResult[T any](record T, metadata map[string]any) ShowResult[T] {
	return ShowResult[T]{Record: record, Metadata: cloneAnyMap(metadata)}
}
