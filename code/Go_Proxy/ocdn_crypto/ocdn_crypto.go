package ocdn_crypto

import (
	"crypto/aes"
	"crypto/cipher"
    "crypto/hmac"
	"crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "crypto/x509"
    "encoding/asn1"
    "encoding/base64"
    "encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
    "os"
	"../loadrsakeys"
)

func DecryptAES(cipherstring string, keystring string) string {
	// Byte array of the string
	ciphertext := []byte(cipherstring)

	// Key
	key := []byte(keystring)

	// Create the AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// Before even testing the decryption,
	// if the text is too small, then it is incorrect
	if len(ciphertext) < aes.BlockSize {
		panic("Text is too short")
	}

	// Get the 16 byte IV
	iv := ciphertext[:aes.BlockSize]

	// Remove the IV from the ciphertext
	ciphertext = ciphertext[aes.BlockSize:]

	// Return a decrypted stream
	stream := cipher.NewCFBDecrypter(block, iv)

	// Decrypt bytes from ciphertext
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext)
}

func EncryptAES(plainstring string, keystring string) string {
	// Byte array of the string
	plaintext := []byte(plainstring)

	// Key
	key := []byte(keystring)

	// Create the AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// Empty array of 16 + plaintext length
	// Include the IV at the beginning
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))

	// Slice of first 16 bytes
	iv := ciphertext[:aes.BlockSize]

	// Write 16 rand bytes to fill iv
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err)
	}

	// Return an encrypted stream
	stream := cipher.NewCFBEncrypter(block, iv)

	// Encrypt bytes from plaintext to ciphertext
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return string(ciphertext)
}

func GenerateMAC(url string, keystring string) string {
    key := []byte(keystring)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(url))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func GenerateSessionKey() string {
    key := make([]byte, 32)
	rand.Read(key)
    return string(key)
}

func GenerateAsymmetricKeys() {
    reader := rand.Reader
	bitSize := 2048
    key, _ := rsa.GenerateKey(reader, bitSize)
    publicKey := key.PublicKey

    // save private key to file
    outFile, _ := os.Create("private.pem")
	defer outFile.Close()
	var privateKey = &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	pem.Encode(outFile, privateKey)

    // save public key to file
    asn1Bytes, _ := asn1.Marshal(publicKey)
	var pemkey = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: asn1Bytes,
	}
	pemfile, _ := os.Create("public.pem")
	defer pemfile.Close()
	pem.Encode(pemfile, pemkey)
}

func ReadPublicKey() *rsa.PublicKey {
    pub_bytes, err := ioutil.ReadFile("pub_test.pem")
if err != nil {
		fmt.Println(err)
		os.Exit(1)
}
    pub_pem := string(pub_bytes)
    pub_parsed, _ := loadrsakeys.ParseRsaPublicKeyFromPemStr(pub_pem)
	
    return pub_parsed
 
}

func ReadPrivateKey() *rsa.PrivateKey {
    priv_bytes, err := ioutil.ReadFile("priv_test.pem")
if err != nil {
		fmt.Println(err)
		os.Exit(1)
}

    priv_pem := string(priv_bytes)

    priv_parsed, _ := loadrsakeys.ParseRsaPrivateKeyFromPemStr(priv_pem)
    return priv_parsed
}

func EncryptAsymmetric(plainstring string, publickey *rsa.PublicKey) string {
    encryptedmsg, _ := rsa.EncryptPKCS1v15(rand.Reader, publickey, []byte(plainstring))
    return string(encryptedmsg)
}

func DecryptAsymmetric(cipherstring string, privatekey *rsa.PrivateKey) string {
    decryptedmsg, err := rsa.DecryptPKCS1v15(rand.Reader, privatekey, []byte(cipherstring))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
    return string(decryptedmsg)
}

func writeToFile(data, file string) {
	ioutil.WriteFile(file, []byte(data), 777)
}

func readFromFile(file string) ([]byte, error) {
	data, err := ioutil.ReadFile(file)
	return data, err
}

/*
func main() {

	content := []byte("Hello this is Annie!!")
    file := "test_file.txt"
	key := generateSessionKey()
    url := "www.foo.com"

    //generateAsymmetricKeys()

    test_case := "rsa"

    if test_case == "mac" {
        mac := generateMAC(url, key)
        fmt.Println(string(mac))    
    } else if test_case == "rsa" {
        priv := readPrivateKey()
        pub := readPublicKey()

        encrypted := encryptAsymmetric(string(content), pub)
        writeToFile(encrypted, file+".rsa.enc")
    
        decrypted := decryptAsymmetric(encrypted, priv)
	    writeToFile(decrypted, file+"rsa.dec")
    } else if test_case == "aes" {
		encrypted := encryptAES(string(content), key)
		writeToFile(encrypted, file+".enc")

		decrypted := decryptAES(string(encrypted), key)
		writeToFile(decrypted, file+".dec")
    }
}*/
