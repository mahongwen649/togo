package coreclient

import (
	"errors"
	"net/http"
)

// RelayRoundTrip is internal-only. It never attaches the Core administrator key,
// follows redirects, or buffers a streaming response in the management adapter.
func (a *HTTPAdapter) RelayRoundTrip(request *http.Request) (*http.Response, error) {
	u := *a.baseURL
	if u.Path != "" && u.Path != "/" {
		return nil, errors.New("relay Core URL must be an origin")
	}
	u.Path = request.URL.Path
	u.RawPath = ""
	u.RawQuery = request.URL.RawQuery
	request = request.Clone(request.Context())
	request.URL = &u
	request.RequestURI = ""
	request.Host = u.Host
	transport := a.client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(request)
}
