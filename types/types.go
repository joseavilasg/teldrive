package types

type AppError struct {
	Error error
	Code  int
}
type Part struct {
	Start int64
	End   int64
	Url   string
	Size  int64
}
