package azurerm

type contextKey string

const (
	ctxKeyCreateOpts contextKey = "azurerm/create-opts"
	ctxKeyCertPath   contextKey = "azurerm/cert-path"
)
