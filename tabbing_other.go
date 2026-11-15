//go:build !darwin

package main

var onSelectAll, onCopyAll func()
var onMenu func(id string)

func routeSelectAll() {}

func shareNative(string) {}
func printNative()       {}
