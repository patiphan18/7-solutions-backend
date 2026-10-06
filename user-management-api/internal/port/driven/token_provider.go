package driven

type TokenProvider interface {
	Issue(subject string) (string, error)
	Validate(token string) (string, error)
}
