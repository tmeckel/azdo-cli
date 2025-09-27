package git

import (
	"net/url"
	"strings"
)

func IsURL(u string) bool {
	return strings.HasPrefix(u, "git@") || isSupportedProtocol(u)
}

func IsSupportedProtocol(u *url.URL) bool {
	return isSupportedProtocol(u.Scheme)
}

func isSupportedProtocol(u string) bool {
	return strings.HasPrefix(u, "ssh") ||
		strings.HasPrefix(u, "git+ssh") ||
		strings.HasPrefix(u, "git") ||
		strings.HasPrefix(u, "http") ||
		strings.HasPrefix(u, "https") ||
		strings.HasPrefix(u, "git+https") ||
		strings.HasPrefix(u, "git+http")
}

func isPossibleProtocol(u string) bool {
	return isSupportedProtocol(u) ||
		strings.HasPrefix(u, "ftp") ||
		strings.HasPrefix(u, "ftps") ||
		strings.HasPrefix(u, "file")
}

// ParseURL normalizes git remote urls
func ParseURL(rawURL string) (u *url.URL, err error) {
	if strings.HasPrefix(rawURL, "git@") {
		// Support scp-like syntax for ssh protocol.
		rawURL = "ssh://" + rawURL
	}

	if strings.HasPrefix(rawURL, "ssh://") {
		items := []rune(rawURL)
		rawURL = "ssh://" + strings.Replace(string(items[len("ssh://"):]), ":", "/", 1)
	} else if !isPossibleProtocol(rawURL) &&
		strings.ContainsRune(rawURL, ':') &&
		// not a Windows path
		!strings.ContainsRune(rawURL, '\\') {
		// support scp-like syntax for ssh protocol
		rawURL = "ssh://" + strings.Replace(rawURL, ":", "/", 1)
	}

	u, err = url.Parse(rawURL)
	if err != nil {
		return u, err
	}

	if strings.EqualFold(u.Scheme, "git+ssh") {
		u.Scheme = "ssh"
	}

	if strings.EqualFold(u.Scheme, "git+https") {
		u.Scheme = "https"
	}

	if !strings.EqualFold(u.Scheme, "ssh") {
		return u, err
	}

	if strings.HasPrefix(u.Path, "//") {
		u.Path = strings.TrimPrefix(u.Path, "/")
	}

	if idx := strings.Index(u.Host, ":"); idx >= 0 {
		u.Host = u.Host[0:idx]
	}

	return u, err
}
