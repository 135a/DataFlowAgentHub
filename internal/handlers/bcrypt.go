package handlers

import "golang.org/x/crypto/bcrypt"

func compareBcrypt(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
