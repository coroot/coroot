package utils

import gonanoid "github.com/matoous/go-nanoid"

func NanoId(size int) string {
	id, _ := gonanoid.Generate("0123456789abcdefghijklmnopqrstuvwxyz", size)
	return id
}

func RandomString(size int) string {
	id, _ := gonanoid.ID(size)
	return id
}
