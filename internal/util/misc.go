package util

import "regexp"

var SemverRegex = regexp.MustCompile(`\bv?(\d+)\.(\d+)\.(\d+)(?:-([\da-z\-]+(?:\.[\da-z\-]+)*))?(?:\+([\da-z\-]+(?:\.[\da-z\-]+)*))?\b`)
