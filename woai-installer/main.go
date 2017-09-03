package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"io"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	key := []byte{
		0xf8, 0x93, 0xab, 0xdd, 0xf3, 0x9b, 0x02, 0x6d,
		0x5d, 0x13, 0x5a, 0x61, 0xfe, 0xcb, 0x91, 0x0f,
		0xe0, 0x69, 0x0a, 0x47, 0xe7, 0xd5, 0x91, 0x83,
		0x9d, 0xdf, 0xc9, 0x70, 0x03, 0x05, 0x3c, 0x5c,
	}
	iv := []byte{
		0x27, 0xc9, 0x1e, 0x10, 0x6f, 0x27, 0x9e, 0xd0,
		0x4d, 0x64, 0xd4, 0x19, 0xcb, 0x74, 0x19, 0xb0,
	}

	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal(err)
	}

	dec := cipher.NewCBCDecrypter(block, iv)
	dec.CryptBlocks(b, b)

	f, err := os.Open("Air Berlin.woai")
	if err != nil {
		log.Fatal(err)
	}
	want := make([]byte, 1024)
	_, err = io.ReadFull(f, want)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(bytes.Equal(want, b[:len(want)]))
}
