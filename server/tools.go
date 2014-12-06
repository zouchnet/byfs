package main

import (
	"io"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
)

func tokenAuth(str string, token string) bool {
	if len(token) <= 32 {
		return false
	}

	hash := token[:32]
	salt := token[32:]

	h := md5.New()
	io.WriteString(h, *password)
	io.WriteString(h, str)
	io.WriteString(h, salt)
	_d := h.Sum(nil)

	_hash := hex.EncodeToString(_d)

	return hash == _hash
}

func randString() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	h := md5.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}


