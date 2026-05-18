package auth

func ValidateSession(token string) bool {
	return token != ""
}
