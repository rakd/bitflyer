package bitflyer

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const (
	// URL is a bitFlyer Lightning API base URL.
	URL            = "https://api.bitflyer.jp"
	minuteToExpire = 525600
	timeInForce    = "GTC"
)

// APIClient struct represents bitFlyer Lightning API client.
type APIClient struct {
	key    string
	secret string
	client *http.Client
}

// AskBid represents bitFlyer Lightning order book ask or bid record.
type AskBid struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}

// OrderBook represents bitFlyer Lightning order book.
type OrderBook struct {
	MidPrice float64  `json:"mid_price"`
	Bids     []AskBid `json:"bids"`
	Asks     []AskBid `json:"asks"`
}

// AssetBalance represents bitFlyer Lightning asset balance.
type AssetBalance []Balance

// Balance represents bitFlyer Lightning asset balance record.
type Balance struct {
	CurrencyCode string  `json:"currency_code"`
	Amount       float64 `json:"amount"`
	Available    float64 `json:"available"`
}

// Ticker represents bitFlyer Lightning ticker.
type Ticker struct {
	ProductCode     string  `json:"product_code"`
	Timestamp       string  `json:"timestamp"`
	TickID          int     `json:"tick_id"`
	BestBid         float64 `json:"best_bid"`
	BestAsk         float64 `json:"best_ask"`
	BestBidSize     float64 `json:"best_bid_size"`
	BestAskSize     float64 `json:"best_ask_size"`
	TotalBidDepth   float64 `json:"total_bid_depth"`
	TotalAskDepth   float64 `json:"total_ask_depth"`
	LTP             float64 `json:"ltp"`
	Volume          float64 `json:"volume"`
	VolumeByProduct float64 `json:"volume_by_product"`
}

// ErrorResponse ...
// e.g. {"status":-500,"error_message":"Invalid signature","data":null}
// e.g. {"status":-500,"error_message":"Key not found","data":null}
type ErrorResponse struct {
	Status       int64  `json:"status"`
	ErrorMessage string `json:"error_message"`
}

// Error ...
func (e *ErrorResponse) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("status=%d, message=%s", e.Status, e.ErrorMessage)
	}
	return e.ErrorMessage

}

// Order represents a new child order.
type Order struct {
	ID                     int     `json:"id"`
	ChildOrderAcceptanceID string  `json:"child_order_acceptance_id"`
	ProductCode            string  `json:"product_code"`
	ChildOrderType         string  `json:"child_order_type"`
	Side                   string  `json:"side"`
	Price                  float64 `json:"price"`
	Size                   float64 `json:"size"`
	MinuteToExpires        int     `json:"minute_to_expire"`
	TimeInForce            string  `json:"time_in_force"`
	Status                 int     `json:"status"`
	ErrorMessage           string  `json:"error_message"`
	AveragePrice           float64 `json:"average_price"`
	ChildOrderState        string  `json:"child_order_state"`
	ExpireDate             string  `json:"expire_date"`
	ChildOrderDate         string  `json:"child_order_date"`
	OutstandingSize        float64 `json:"outstanding_size"`
	CancelSize             float64 `json:"cancel_size"`
	ExecutedSize           float64 `json:"executed_size"`
	TotalCommission        float64 `json:"total_commission"`
}

// New creates a new bitFlyer Lightning API client.
func New(key, secret string) (client *APIClient) {
	client = new(APIClient)
	client.key = key
	client.secret = secret
	client.client = new(http.Client)
	return client
}

// GetOrderBook returns bitFlyer Lightning order book.
func (api APIClient) GetOrderBook() (orderBook OrderBook, resp []byte, err error) {
	resp, err = api.doGetRequest("/v1/getboard", map[string]string{}, []byte(""), &orderBook)
	if err != nil {
		return orderBook, resp, err
	}
	return orderBook, resp, nil
}

// GetBalance returns bitFlyer Lightning account asset balance.
func (api APIClient) GetBalance() (assetBalance AssetBalance, resp []byte, err error) {
	resp, err = api.doGetRequest("/v1/me/getbalance", map[string]string{}, []byte(""), &assetBalance)
	if err != nil {
		return assetBalance, resp, err
	}
	return assetBalance, resp, nil
}

// GetTicker returns bitFlyer Lightning ticker.
func (api APIClient) GetTicker() (ticker Ticker, resp []byte, err error) {
	resp, err = api.doGetRequest("/v1/getticker", map[string]string{}, []byte(""), &ticker)
	if err != nil {
		return ticker, resp, err
	}
	return ticker, resp, nil
}

// NewOrder sends a new order.
func (api APIClient) NewOrder(order Order) (newOrder Order, resp []byte, err error) {
	newOrder = order
	if newOrder.MinuteToExpires <= 0 {
		newOrder.MinuteToExpires = minuteToExpire
	}
	if newOrder.TimeInForce == "" {
		newOrder.TimeInForce = timeInForce
	}
	data, err := json.Marshal(newOrder)
	if err != nil {
		return newOrder, resp, err
	}
	resp, err = api.doPostRequest("/v1/me/sendchildorder", map[string]string{}, data, &newOrder)
	if err != nil {
		return newOrder, resp, err
	}
	if newOrder.ErrorMessage != "" {
		return newOrder, resp, errors.New(newOrder.ErrorMessage)
	}
	return newOrder, resp, nil
}

// GetOrders returns bitFlyer Lightning orders.
func (api APIClient) GetOrders(query map[string]string) (orders []Order, resp []byte, err error) {
	resp, err = api.doGetRequest("/v1/me/getchildorders", query, []byte(""), &orders)
	if err != nil {
		return orders, resp, err
	}
	return orders, resp, nil
}

func (api *APIClient) doGetRequest(endpoint string, query map[string]string, body []byte, data interface{}) (resp []byte, err error) {
	resp, err = api.doRequest("GET", endpoint, query, body)
	if err != nil {
		return resp, err
	}
	jerr := json.Unmarshal(resp, data)
	if jerr != nil {
		errRes := ErrorResponse{}
		jerr = json.Unmarshal(resp, &errRes)
		if jerr != nil {
			return resp, err
		}
		err = &errRes
		return resp, err
	}
	return resp, nil
}

func (api *APIClient) doPostRequest(endpoint string, query map[string]string, body []byte, data interface{}) (resp []byte, err error) {
	resp, err = api.doRequest("POST", endpoint, query, body)
	if err != nil {
		return resp, err
	}
	jerr := json.Unmarshal(resp, data)
	if jerr != nil {
		return resp, err
	}
	return resp, nil
}

func (api *APIClient) doRequest(method, endpoint string, query map[string]string, data []byte) ([]byte, error) {
	req, err := http.NewRequest(method, URL+endpoint, bytes.NewBuffer(data))
	if err != nil {
		return nil, requestError(err.Error())
	}
	q := req.URL.Query()
	for key, value := range query {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()
	headers := headers(api.key, api.secret, method, req.URL.RequestURI(), string(data))
	setHeaders(req, headers)
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, requestError(err.Error())
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, requestError(err.Error())
	}
	return body, nil
}

func headers(key, secret, method, uri, body string) map[string]string {
	timestamp := strconv.Itoa(int(time.Now().Unix()))
	message := timestamp + method + uri + body
	signature := computeHmac256(message, secret)
	headers := map[string]string{
		"Content-Type":     "application/json",
		"ACCESS-KEY":       key,
		"ACCESS-TIMESTAMP": timestamp,
		"ACCESS-SIGN":      signature,
	}
	return headers
}

func computeHmac256(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func requestError(err interface{}) error {
	return fmt.Errorf("Could not execute request! (%s)", err)
}

func setHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Add(key, value)
	}
}
