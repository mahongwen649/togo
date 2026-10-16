package httpapi

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AccountPaymentConfig deliberately keeps Portal settlement separate from Core.
// The stable secret encrypts pending orders' merchant configuration snapshots.
type AccountPaymentConfig struct {
	PublicURL  string
	Secret     string
	ProviderID int64
}

type accountPaymentProvider struct {
	ID           int64
	PID          string
	Key          string
	Base         string
	CID          string
	Min          int64
	Max          int64
	Daily        int64
	ChannelDaily int64
	Fee          string
	Pending      int
	Timeout      int
}

var errAccountPaymentUnavailable = errors.New("账号套餐在线充值暂不可用，请稍后重试")

func paymentCents(raw string) (int64, error) {
	return paymentCentsLimit(raw, 100000000)
}

func paymentCentsLimit(raw string, limit int64) (int64, error) {
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || len(parts[0]) == 0 || len(parts[0]) > 10 {
		return 0, errors.New("金额格式无效")
	}
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return 0, errors.New("金额格式无效")
			}
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	var fraction int64
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, errors.New("金额最多保留两位小数")
		}
		fraction, _ = strconv.ParseInt((parts[1] + "00")[:2], 10, 64)
	}
	cents := whole*100 + fraction
	if cents <= 0 || cents > limit {
		return 0, errors.New("充值金额超出允许范围")
	}
	return cents, nil
}

func centsText(cents int64) string { return fmt.Sprintf("%d.%02d", cents/100, cents%100) }

// EasyPay/Core express fees in percent; round the fee upward to the next cent.
func accountPaymentTotal(cents int64, rate string) (int64, error) {
	r, ok := new(big.Rat).SetString(rate)
	if !ok || r.Sign() < 0 || r.Cmp(big.NewRat(100, 1)) > 0 {
		return 0, errAccountPaymentUnavailable
	}
	r.Mul(r, big.NewRat(cents, 100))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if rem.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	return cents + q.Int64(), nil
}

// Matches the existing Core EasyPay provider's sorted, unescaped MD5 protocol.
func accountPaymentSign(values url.Values, key string) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		if k != "sign" && k != "sign_type" && values.Get(k) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+values.Get(k))
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(sum[:])
}

func validateAccountPaymentSignature(values url.Values, key string) bool {
	for _, v := range values {
		if len(v) != 1 {
			return false
		}
	}
	if values.Get("sign_type") != "" && values.Get("sign_type") != "MD5" {
		return false
	}
	return hmac.Equal([]byte(accountPaymentSign(values, key)), []byte(strings.ToLower(values.Get("sign"))))
}

func paymentURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errAccountPaymentUnavailable
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")) {
		return nil, errAccountPaymentUnavailable
	}
	return u, nil
}

func (s *Server) paymentCipher() (cipher.AEAD, error) {
	if len(s.accountPayment.Secret) < 32 {
		return nil, errAccountPaymentUnavailable
	}
	key := sha256.Sum256([]byte("portal-account-payment-v1:" + s.accountPayment.Secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Server) sealPayment(p accountPaymentProvider, trade string) ([]byte, error) {
	aead, err := s.paymentCipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, data, []byte(trade)), nil
}

func (s *Server) openPayment(data []byte, trade string) (accountPaymentProvider, error) {
	var p accountPaymentProvider
	aead, err := s.paymentCipher()
	if err != nil || len(data) < aead.NonceSize() {
		return p, errAccountPaymentUnavailable
	}
	plain, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte(trade))
	if err != nil {
		return p, errAccountPaymentUnavailable
	}
	err = json.Unmarshal(plain, &p)
	return p, err
}

func (s *Server) loadAccountPaymentProvider(ctx context.Context) (accountPaymentProvider, error) {
	p := accountPaymentProvider{Min: 100, Max: 100000000, Fee: "0", Pending: 3, Timeout: 30}
	if s.portalDB == nil || s.corePaymentDB == nil {
		return p, errAccountPaymentUnavailable
	}
	if _, err := s.paymentCipher(); err != nil {
		return p, err
	}
	if _, err := paymentURL(s.accountPayment.PublicURL); err != nil {
		return p, err
	}
	rows, err := s.corePaymentDB.QueryContext(ctx, `SELECT key,value FROM settings WHERE key IN ('payment_enabled','payment_visible_method_alipay_enabled','payment_visible_method_alipay_source','MIN_RECHARGE_AMOUNT','MAX_RECHARGE_AMOUNT','DAILY_RECHARGE_LIMIT','RECHARGE_FEE_RATE','MAX_PENDING_ORDERS','ORDER_TIMEOUT_MINUTES')`)
	if err != nil {
		return p, errAccountPaymentUnavailable
	}
	settings := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return p, errAccountPaymentUnavailable
		}
		settings[k] = v
	}
	err = rows.Err()
	rows.Close()
	if err != nil || settings["payment_enabled"] != "true" {
		return p, errAccountPaymentUnavailable
	}
	source := strings.ToLower(strings.TrimSpace(settings["payment_visible_method_alipay_source"]))
	if source != "" && ((source != "easypay_alipay" && source != "easypay") || settings["payment_visible_method_alipay_enabled"] != "true") {
		return p, errAccountPaymentUnavailable
	}
	if source == "" {
		// Core's legacy routing uses the sole enabled provider family when no source is configured.
		var families int
		if err := s.corePaymentDB.QueryRowContext(ctx, `SELECT count(DISTINCT provider_key) FROM payment_provider_instances WHERE enabled=true AND 'alipay'=ANY(string_to_array(replace(supported_types,' ',''),','))`).Scan(&families); err != nil || families != 1 {
			return p, errAccountPaymentUnavailable
		}
	}
	for key, dest := range map[string]*int64{"MIN_RECHARGE_AMOUNT": &p.Min, "MAX_RECHARGE_AMOUNT": &p.Max, "DAILY_RECHARGE_LIMIT": &p.Daily} {
		if v := settings[key]; v != "" && v != "0" {
			n, e := paymentCents(v)
			if e != nil {
				return p, errAccountPaymentUnavailable
			}
			*dest = n
		}
	}
	if v := settings["RECHARGE_FEE_RATE"]; v != "" {
		p.Fee = v
	}
	if _, err := accountPaymentTotal(100, p.Fee); err != nil {
		return p, err
	}
	for key, dest := range map[string]*int{"MAX_PENDING_ORDERS": &p.Pending, "ORDER_TIMEOUT_MINUTES": &p.Timeout} {
		if v := settings[key]; v != "" {
			n, e := strconv.Atoi(v)
			if e != nil || n <= 0 || n > 1440 {
				return p, errAccountPaymentUnavailable
			}
			*dest = n
		}
	}
	var stored, limits string
	err = s.corePaymentDB.QueryRowContext(ctx, `SELECT id,config,limits FROM payment_provider_instances WHERE enabled=true AND provider_key='easypay' AND 'alipay'=ANY(string_to_array(replace(supported_types,' ',''),',')) AND ($1::bigint=0 OR id=$1) ORDER BY sort_order,id LIMIT 1`, s.accountPayment.ProviderID).Scan(&p.ID, &stored, &limits)
	if err != nil {
		return p, errAccountPaymentUnavailable
	}
	var config map[string]string
	// Current Core stores JSON. Legacy ciphertext must be re-saved through Core.
	if json.Unmarshal([]byte(stored), &config) != nil {
		return p, errAccountPaymentUnavailable
	}
	if currency := strings.ToUpper(strings.TrimSpace(config["currency"])); currency != "" && currency != "CNY" {
		return p, errAccountPaymentUnavailable
	}
	p.PID, p.Key, p.Base, p.CID = config["pid"], config["pkey"], strings.TrimRight(config["apiBase"], "/"), config["cidAlipay"]
	if p.CID == "" {
		p.CID = config["cid"]
	}
	for _, suffix := range []string{"/submit.php", "/mapi.php", "/api.php"} {
		p.Base = strings.TrimSuffix(p.Base, suffix)
	}
	if p.PID == "" || p.Key == "" {
		return p, errAccountPaymentUnavailable
	}
	if _, err = paymentURL(p.Base); err != nil {
		return p, err
	}
	if limits != "" {
		var channels map[string]struct {
			Min   json.Number `json:"singleMin"`
			Max   json.Number `json:"singleMax"`
			Daily json.Number `json:"dailyLimit"`
		}
		if json.Unmarshal([]byte(limits), &channels) != nil {
			return p, errAccountPaymentUnavailable
		}
		limit := channels["alipay"]
		for _, entry := range []struct {
			value json.Number
			dest  *int64
		}{{limit.Min, &p.Min}, {limit.Max, &p.Max}, {limit.Daily, &p.ChannelDaily}} {
			value, dest := entry.value, entry.dest
			if value == "" || value == "0" {
				continue
			}
			n, e := paymentCents(string(value))
			if e != nil {
				return p, errAccountPaymentUnavailable
			}
			if dest == &p.Min {
				if n > p.Min {
					p.Min = n
				}
			} else if dest == &p.Max {
				if n < p.Max {
					p.Max = n
				}
			} else {
				*dest = n
			}
		}
	}
	return p, nil
}

func (s *Server) accountPaymentURL(p accountPaymentProvider, trade string, pay int64, mobile bool) string {
	base := strings.TrimRight(s.accountPayment.PublicURL, "/")
	v := url.Values{"pid": {p.PID}, "type": {"alipay"}, "out_trade_no": {trade}, "money": {centsText(pay)}, "name": {"TogoAPI 账号套餐余额充值"}, "notify_url": {base + "/api/v1/account-market/payments/webhook/easypay"}, "return_url": {base + "/payment/result"}}
	if p.CID != "" {
		v.Set("cid", p.CID)
	}
	if mobile {
		v.Set("device", "mobile")
	}
	v.Set("sign", accountPaymentSign(v, p.Key))
	v.Set("sign_type", "MD5")
	return p.Base + "/submit.php?" + v.Encode()
}

func queryAccountPayment(ctx context.Context, p accountPaymentProvider, trade string) (url.Values, bool, error) {
	if _, err := paymentURL(p.Base); err != nil {
		return nil, false, err
	}
	v := url.Values{"act": {"order"}, "pid": {p.PID}, "key": {p.Key}, "out_trade_no": {trade}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.Base+"/api.php?"+v.Encode(), nil)
	if err != nil {
		return nil, false, errAccountPaymentUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, errAccountPaymentUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, false, errAccountPaymentUnavailable
	}
	var data map[string]json.RawMessage
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data) != nil {
		return nil, false, errAccountPaymentUnavailable
	}
	var code int
	if json.Unmarshal(data["code"], &code) != nil || code != 1 {
		return nil, false, nil
	}
	if nested, ok := data["data"]; ok {
		var inner map[string]json.RawMessage
		if json.Unmarshal(nested, &inner) == nil {
			for k, val := range inner {
				if _, ok := data[k]; !ok {
					data[k] = val
				}
			}
		}
	}
	value := func(key string) string {
		raw := data[key]
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return text
		}
		return string(raw)
	}
	tradeStatus, status := value("trade_status"), value("status")
	if (tradeStatus != "" && tradeStatus != "TRADE_SUCCESS") || (status != "" && status != "1") || (tradeStatus == "" && status == "") {
		return nil, false, nil
	}
	if out := value("out_trade_no"); out != "" && out != trade {
		return nil, false, errAccountPaymentUnavailable
	}
	if pid := value("pid"); pid != "" && pid != p.PID {
		return nil, false, errAccountPaymentUnavailable
	}
	if kind := value("type"); kind != "" && kind != "alipay" {
		return nil, false, errAccountPaymentUnavailable
	}
	return url.Values{"out_trade_no": {trade}, "trade_no": {value("trade_no")}, "money": {value("money")}, "pid": {p.PID}, "type": {"alipay"}, "trade_status": {"TRADE_SUCCESS"}}, true, nil
}
