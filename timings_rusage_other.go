//go:build !unix

package clicky

func processUsage() ProcessUsage { return ProcessUsage{} }
