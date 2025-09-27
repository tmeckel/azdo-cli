package azdo

type Authenticator interface {
	GetPAT(organizationName string) (string, error)
	GetAuthorizationHeader(organizationName string) (string, error)
}
