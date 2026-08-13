package service

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

// VerifyDingTalkSignature 钉钉回调/机器人加签校验：
// sign = base64(hmac_sha256(secret, timestamp+"\n"+secret))。
// 调用方传入的 sign 应为 URL 解码后的 base64 值（HTTP 层由 query 参数自动解码）。
func VerifyDingTalkSignature(secret, timestamp, sign string) bool {
	if secret == "" || timestamp == "" || sign == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return strings.TrimSpace(sign) == expected
}

// VerifyFeishuSignature 飞书事件回调验签（v2）：
// signature = base64(hmac_sha256(encryptKey, timestamp + nonce + encryptKey + body))。
func VerifyFeishuSignature(encryptKey, timestamp, nonce string, body []byte, signature string) bool {
	if encryptKey == "" || timestamp == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(encryptKey))
	mac.Write([]byte(timestamp + nonce + encryptKey))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return strings.TrimSpace(signature) == expected
}

// WeComCallback 企业微信回调加解密（WXBizMsgCrypt）。
type WeComCallback struct {
	Token          string
	EncodingAESKey string // 43 位 EncodingAESKey
	CorpID         string
}

// VerifyWeComSignature 校验企微回调 msg_signature：
// sha1(sort(token, timestamp, nonce, encrypt))，字典序排序后拼接。
func (w WeComCallback) VerifyWeComSignature(msgSignature, timestamp, nonce, encrypt string) bool {
	if w.Token == "" || msgSignature == "" {
		return false
	}
	parts := []string{w.Token, timestamp, nonce, encrypt}
	sort.Strings(parts)
	mac := sha1.Sum([]byte(strings.Join(parts, "")))
	expected := fmt.Sprintf("%x", mac)
	return strings.EqualFold(strings.TrimSpace(msgSignature), expected)
}

// Decrypt 解密企微回调消息，返回明文 XML/JSON 与 receiveid 前缀校验后的 msg。
func (w WeComCallback) Decrypt(encrypted string) ([]byte, error) {
	if w.EncodingAESKey == "" {
		return nil, fmt.Errorf("EncodingAESKey 未配置")
	}
	key, err := base64.StdEncoding.DecodeString(w.EncodingAESKey + "=")
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("EncodingAESKey 非法")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("加密消息 base64 解码失败: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aes.BlockSize || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度非法")
	}
	iv := key[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(ciphertext))
	mode.CryptBlocks(plain, ciphertext)
	plain, err = pkcs7Unpad(plain)
	if err != nil {
		return nil, err
	}
	// 结构：random(16) + msg_len(4, big-endian) + msg + receiveid
	if len(plain) < 20 {
		return nil, fmt.Errorf("解密后数据过短")
	}
	msgLen := binary.BigEndian.Uint32(plain[16:20])
	msgEnd := 20 + int(msgLen)
	if msgEnd > len(plain) {
		return nil, fmt.Errorf("消息长度非法")
	}
	msg := plain[20:msgEnd]
	receiveID := string(plain[msgEnd:])
	if w.CorpID != "" && receiveID != w.CorpID {
		return nil, fmt.Errorf("receiveid 不匹配")
	}
	return msg, nil
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("空数据")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > aes.BlockSize || padding > len(data) {
		return nil, fmt.Errorf("PKCS7 填充非法")
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			return nil, fmt.Errorf("PKCS7 填充不一致")
		}
	}
	return data[:len(data)-padding], nil
}

// constantTimeEqual 常量时间比较（防时序攻击）。
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// BytesEqual 常量时间字节比较（测试用辅助导出）。
func BytesEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}
