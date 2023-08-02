package util

import (
	"os"
)

func IsDebugEnabled() (bool, string) {
	debugValue, isDebugSet := os.LookupEnv("AZDO_DEBUG")
	if !isDebugSet {
		return false, ""
	}
	switch debugValue {
	case "false", "0", "no", "":
		return false, debugValue
	default:
		return true, debugValue
	}
}
