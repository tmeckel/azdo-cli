package util

import (
	"net/url"

	"github.com/PuerkitoBio/purell"
)

// URLComparer defines methods to safely compare URL strings and url.URL instances.
type URLComparer interface {
	// EqualStrings compares two URL strings after parsing and normalization.
	EqualStrings(a, b string) (bool, error)
	// EqualURLs compares two url.URL instances after normalization.
	EqualURLs(a, b *url.URL) bool
}

// DefaultURLComparer is the default implementation of URLComparer.
type DefaultURLComparer struct{}

// NewURLComparer returns a new DefaultURLComparer.
func NewURLComparer() URLComparer {
	return DefaultURLComparer{}
}

// EqualStrings parses and compares two URL strings.
func (c DefaultURLComparer) EqualStrings(a, b string) (bool, error) {
	if (a == "") != (b == "") {
		return false, nil
	}
	ua, err := url.Parse(a)
	if err != nil {
		return false, err
	}
	ub, err := url.Parse(b)
	if err != nil {
		return false, err
	}
	return c.EqualURLs(ua, ub), nil
}

// EqualURLs compares two url.URL instances for equality after normalization.
func (c DefaultURLComparer) EqualURLs(a, b *url.URL) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	a_c := *a
	a_c.User = nil
	b_c := *b
	b_c.User = nil
	flags := purell.FlagsUsuallySafeGreedy | purell.FlagSortQuery
	return purell.NormalizeURL(&a_c, flags) == purell.NormalizeURL(&b_c, flags)
}
