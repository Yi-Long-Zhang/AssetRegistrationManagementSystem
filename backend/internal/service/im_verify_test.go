package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestVerifyDingTalkSignature(t *testing.T) {
	secret := "SEC-test-secret"
	timestamp := "1723456789000"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !VerifyDingTalkSignature(secret, timestamp, sign) {
		t.Fatal("valid dingtalk sign should pass")
	}
	if VerifyDingTalkSignature(secret, timestamp, "bad") {
		t.Fatal("wrong sign should fail")
	}
	if VerifyDingTalkSignature("", timestamp, sign) {
		t.Fatal("empty secret should fail")
	}
}

func TestVerifyFeishuSignature(t *testing.T) {
	key := "lark-encrypt-key"
	timestamp := "1723456789"
	nonce := "abcdef"
	body := []byte(`{"action":"approve","ticketId":1}`)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(timestamp + nonce + key))
	mac.Write(body)
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !VerifyFeishuSignature(key, timestamp, nonce, body, sign) {
		t.Fatal("valid feishu sign should pass")
	}
	if VerifyFeishuSignature(key, timestamp, nonce, []byte(`{"action":"reject"}`), sign) {
		t.Fatal("tampered body should fail")
	}
}

func TestWeComCallbackDecryptRoundTrip(t *testing.T) {
	// 生成 43 位 EncodingAESKey（32 字节 base64 去掉末尾 =）
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatal(err)
	}
	aesKey := strings.TrimRight(base64.StdEncoding.EncodeToString(keyBytes), "=")
	corpID := "ww-test-corp"
	wc := WeComCallback{Token: "test-token", EncodingAESKey: aesKey, CorpID: corpID}

	plain := []byte(`{"action":"approve","ticketId":7}`)
	encrypted := weComEncrypt(t, keyBytes, plain, corpID)

	// msg_signature 校验
	msgSignature := weComMsgSignature("test-token", "1723456789", "nonce", encrypted)
	if !wc.VerifyWeComSignature(msgSignature, "1723456789", "nonce", encrypted) {
		t.Fatal("valid msg_signature should pass")
	}
	if wc.VerifyWeComSignature("bad", "1723456789", "nonce", encrypted) {
		t.Fatal("wrong msg_signature should fail")
	}

	// 解密 round-trip
	decrypted, err := wc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(decrypted) != string(plain) {
		t.Fatalf("decrypt mismatch: %s != %s", decrypted, plain)
	}
}

// weComEncrypt 企微消息加密（测试辅助）：random(16) + msg_len(4) + msg + receiveid，AES-256-CBC + PKCS7。
func weComEncrypt(t *testing.T, key []byte, msg []byte, receiveID string) string {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(msg)))
	plain := append(buf, msgLen...)
	plain = append(plain, msg...)
	plain = append(plain, []byte(receiveID)...)
	plain = pkcs7Pad(plain, aes.BlockSize)

	ciphertext := make([]byte, len(plain))
	iv := key[:aes.BlockSize]
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plain)
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	for i := 0; i < padding; i++ {
		data = append(data, byte(padding))
	}
	return data
}

func weComMsgSignature(token, timestamp, nonce, encrypt string) string {
	parts := []string{token, timestamp, nonce, encrypt}
	sort.Strings(parts)
	mac := sha1.Sum([]byte(strings.Join(parts, "")))
	return fmt.Sprintf("%x", mac)
}
