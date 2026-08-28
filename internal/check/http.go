package check

import (
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 12 * time.Second}

const userAgent = "dispaccio/1.0 (+https://github.com)"

func fetch(url string) (int, []byte) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, body
}
