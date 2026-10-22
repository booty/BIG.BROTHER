package media

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

const ownerHeader = "X-Bigbrother-Session"

// NewProxy exposes only the validated WHIP/WHEP URL shapes under /media/ and
// forwards all signaling directly to the loopback MediaMTX listener.
func NewProxy(manager *Manager, upstreamURL string) (http.Handler, error) {
	upstream, err := url.Parse(upstreamURL)
	if err != nil || upstream.Scheme == "" || upstream.Host == "" {
		return nil, fmt.Errorf("invalid MediaMTX HTTP URL %q", upstreamURL)
	}
	reverseProxy := httputil.NewSingleHostReverseProxy(upstream)
	reverseProxy.Transport = manager.options.HTTPClient.Transport
	baseDirector := reverseProxy.Director
	reverseProxy.Director = func(request *http.Request) {
		baseDirector(request)
		request.URL.Path = strings.TrimPrefix(request.URL.Path, "/media")
		request.URL.RawPath = ""
		request.Host = upstream.Host
	}
	reverseProxy.ModifyResponse = func(response *http.Response) error {
		request := response.Request
		if request == nil {
			return nil
		}
		ownerID := request.Header.Get(ownerHeader)
		if !validStreamID(ownerID) {
			ownerID = streamIDFromPath(request.URL.Path)
		}
		switch request.Method {
		case http.MethodPost:
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				location := response.Header.Get("Location")
				resource, rewritten, err := rewriteLocation(location, upstream)
				if err != nil {
					return err
				}
				manager.recordSession(ownerID, resource)
				response.Header.Set("Location", rewritten)
			}
		case http.MethodDelete:
			if response.StatusCode >= 200 && response.StatusCode < 300 || response.StatusCode == http.StatusNotFound {
				manager.forgetSession(ownerID, request.URL.RequestURI())
			}
		}
		return nil
	}
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, "MediaMTX signaling proxy failed: "+err.Error(), http.StatusBadGateway)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !manager.Status().Available {
			http.Error(w, "media unavailable: "+manager.Status().Message, http.StatusServiceUnavailable)
			return
		}
		if !validProxyRequest(request) {
			http.Error(w, "unsupported media signaling request", http.StatusMethodNotAllowed)
			return
		}
		reverseProxy.ServeHTTP(w, request)
	}), nil
}

func validProxyRequest(request *http.Request) bool {
	if !strings.HasPrefix(request.URL.Path, "/media/") || strings.Contains(request.URL.Path, "//") {
		return false
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) < 3 || len(parts) > 4 || parts[0] != "media" || !validStreamID(parts[1]) || (parts[2] != "whip" && parts[2] != "whep") {
		return false
	}
	if len(parts) == 4 && !validResourceID(parts[3]) {
		return false
	}
	switch request.Method {
	case http.MethodOptions:
		return len(parts) == 3 || len(parts) == 4
	case http.MethodPost:
		return len(parts) == 3
	case http.MethodPatch, http.MethodDelete:
		return len(parts) == 4
	default:
		return false
	}
}

func validResourceID(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

func streamIDFromPath(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) >= 1 && validStreamID(parts[0]) {
		return parts[0]
	}
	return ""
}

func rewriteLocation(value string, upstream *url.URL) (resource, rewritten string, err error) {
	location, err := url.Parse(value)
	if err != nil || value == "" {
		return "", "", fmt.Errorf("MediaMTX returned an invalid Location header %q", value)
	}
	if location.IsAbs() && (location.Scheme != upstream.Scheme || location.Host != upstream.Host) {
		return "", "", fmt.Errorf("MediaMTX returned an unexpected Location origin %q", value)
	}
	if location.Host != "" && !location.IsAbs() {
		return "", "", fmt.Errorf("MediaMTX returned an invalid Location host %q", value)
	}
	if !strings.HasPrefix(location.Path, "/") {
		location.Path = "/" + location.Path
	}
	resource = location.RequestURI()
	if !validSessionResource(resource) {
		return "", "", fmt.Errorf("MediaMTX returned an unexpected Location path %q", value)
	}
	return resource, "/media" + resource, nil
}
