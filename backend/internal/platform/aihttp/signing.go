package aihttp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func Sign(req *http.Request, body []byte, secret string) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	req.Header.Set("X-ZBT-Timestamp", timestamp)
	req.Header.Set("X-ZBT-Signature", hex.EncodeToString(mac.Sum(nil)))
}
