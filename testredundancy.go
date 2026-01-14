// Package testredundancy identifies redundant tests in Go projects.
//
// It analyzes test coverage to find the minimal set of tests needed to
// maintain a coverage threshold, identifying tests that don't contribute
// unique coverage beyond a baseline test set.
package testredundancy
