package promtail

import (
	"bytes"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type httpClient struct {
	parent *http.Client
}

func NewHttpClient() *httpClient {
	return &httpClient{
		parent: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Dial: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 5 * time.Second,
				}).Dial,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 7 * time.Second,
				ExpectContinueTimeout: 7 * time.Second,
			},
		},
	}
}

func (client *httpClient) sendJsonReq(method, url string, ctype string, reqBody []byte) (resp *http.Response, resBody []byte, err error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("Connection", "close")
	resp, err = client.parent.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		req.Close = true
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	resBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, resBody, nil
}
