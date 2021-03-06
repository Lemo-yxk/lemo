/**
* @program: kitty
*
* @description:
*
* @author: lemo
*
* @create: 2021-05-21 17:37
**/

package client

import (
	"net/http"
	"net/textproto"
	url2 "net/url"
	"time"

	"github.com/lemoyxk/kitty"
)

type info struct {
	handler         *client
	headerKey       []string
	headerValue     []string
	cookies         []*http.Cookie
	body            interface{}
	progress        *progress
	userName        string
	passWord        string
	clientTimeout   time.Duration
	proxy           func(*http.Request) (*url2.URL, error)
	dialerKeepAlive time.Duration
}

func (h *info) Progress(progress *progress) *info {
	h.progress = progress
	return h
}

func (h *info) Timeout(timeout time.Duration) *info {
	h.clientTimeout = timeout
	return h
}

func (h *info) Proxy(url string) *info {
	var fixUrl, _ = url2.Parse(url)
	h.proxy = http.ProxyURL(fixUrl)
	return h
}

func (h *info) KeepAlive(keepalive time.Duration) *info {
	h.dialerKeepAlive = keepalive
	return h
}

func (h *info) SetBasicAuth(userName, passWord string) *info {
	h.userName = userName
	h.passWord = passWord
	return h
}

func (h *info) SetHeaders(headers map[string]string) *info {
	h.headerKey = nil
	h.headerValue = nil
	for key, value := range headers {
		h.headerKey = append(h.headerKey, textproto.CanonicalMIMEHeaderKey(key))
		h.headerValue = append(h.headerValue, value)
	}
	return h
}

func (h *info) AddHeader(key string, value string) *info {
	h.headerKey = append(h.headerKey, textproto.CanonicalMIMEHeaderKey(key))
	h.headerValue = append(h.headerValue, value)
	return h
}

func (h *info) SetHeader(key string, value string) *info {
	for i := 0; i < len(h.headerKey); i++ {
		if textproto.CanonicalMIMEHeaderKey(h.headerKey[i]) == textproto.CanonicalMIMEHeaderKey(key) {
			h.headerValue[i] = value
			return h
		}
	}

	h.headerKey = append(h.headerKey, key)
	h.headerValue = append(h.headerValue, value)
	return h
}

func (h *info) SetCookies(cookies []*http.Cookie) *info {
	h.cookies = cookies
	return h
}

func (h *info) AddCookie(cookie *http.Cookie) *info {
	for i := 0; i < len(h.cookies); i++ {
		if h.cookies[i].String() == cookie.String() {
			h.cookies[i] = cookie
			return h
		}
	}
	h.cookies = append(h.cookies, cookie)
	return h
}

func (h *info) Json(body ...interface{}) *params {
	h.SetHeader(kitty.ContentType, kitty.ApplicationJson)
	h.body = body
	request, cancel, err := getRequest(h.handler.method, h.handler.url, h)
	if err != nil {
		return &params{err: err}
	}
	return &params{info: h, req: request, cancel: cancel}
}

func (h *info) Query(body ...map[string]interface{}) *params {
	h.body = body
	request, cancel, err := getRequest(h.handler.method, h.handler.url, h)
	if err != nil {
		return &params{err: err}
	}
	return &params{info: h, req: request, cancel: cancel}
}

func (h *info) Form(body ...map[string]interface{}) *params {
	h.SetHeader(kitty.ContentType, kitty.ApplicationFormUrlencoded)
	h.body = body
	request, cancel, err := getRequest(h.handler.method, h.handler.url, h)
	if err != nil {
		return &params{err: err}
	}
	return &params{info: h, req: request, cancel: cancel}
}

func (h *info) Multipart(body ...map[string]interface{}) *params {
	h.SetHeader(kitty.ContentType, kitty.MultipartFormData)
	h.body = body
	request, cancel, err := getRequest(h.handler.method, h.handler.url, h)
	if err != nil {
		return &params{err: err}
	}
	return &params{info: h, req: request, cancel: cancel}
}
