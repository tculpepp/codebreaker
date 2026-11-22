// Package message turns mistakes into short, friendly messages for the
// player, such as "Red is used twice (positions 1 and 4). Each color may
// appear only once."
//
// # Where it fits
//
// Both front ends show these messages: the terminal (package ui) and the
// web app (package web). Keeping the wording here, in a package with no
// input or output of its own, means the two always explain a mistake in
// the same words. It imports only code and game, whose errors it explains.
//
// Errors from other packages can carry their own message by having a
// Friendly method (see Friendly), so this package never needs to know
// about them.
//
// # Go concepts shown here
//
//   - errors.As for picking out an error's concrete type, even when wrapped
//   - errors.As with an interface target
//   - A small package extracted so two others can share it, without
//     either importing the other
//
// Search for "Go note:" to find the explanation of each one.
package message
