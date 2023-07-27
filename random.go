package main

import (
	"crypto/rand"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandString generates random string of n size
// It returns the generated random string.and any write error encountered.
func RandString(n int) string {
	r := make([]byte, n)
	_, err := rand.Read(r)
	if err != nil {
		return ""
	}

	b := make([]byte, n)
	l := len(letters)
	for i := range b {
		b[i] = letters[int(r[i])%l]
	}
	return string(b)
}
