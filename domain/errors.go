package domain

import "errors"

// ErrNoCodeChanges indicates that a code environment contains no changes to process.
var ErrNoCodeChanges = errors.New("no code changes detected")
