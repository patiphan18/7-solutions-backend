package driving

type RegisterCommand struct {
	Name     string
	Email    string
	Password string
}

type LoginCommand struct {
	Email    string
	Password string
}

type UpdateUserCommand struct {
	Name  *string
	Email *string
}

type LoginResult struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int32
}
