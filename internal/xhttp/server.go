package xhttp

import (
	"errors"
	"fmt"
	"github.com/donutloop/httpcache/internal/cache"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
)

func NewProxy(capacity int64) *Proxy {
	return &Proxy{
		cache:  cache.NewLRUCache(capacity),
		client: &http.Client{},
	}
}

type Proxy struct {
	cache  *cache.LRUCache
	client *http.Client
}

func (p *Proxy) ServeHTTP(resp http.ResponseWriter, req *http.Request) {

	proxyResponse, err := p.Do(req)
	if err != nil {
		log.Println(err.Error())
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	for k, vv := range proxyResponse.Header {
		for _, v := range vv {
			resp.Header().Add(k, v)
		}
	}

	body, err := ioutil.ReadAll(proxyResponse.Body)
	if err != nil {
		log.Println(fmt.Sprintf("proxy couldn't read body of response (%v)", err))
		requestDumped, responseDumped, err := dump(req, proxyResponse)
		if err == nil {
			log.Println(fmt.Sprintf("request: %#v", requestDumped))
			log.Println(fmt.Sprintf("response: %#v", responseDumped))
		}
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp.WriteHeader(proxyResponse.StatusCode)
	resp.Write(body)
}

func (p *Proxy) Do(req *http.Request) (*http.Response, error) {
	cachedResponse, ok := p.cache.Get(req.URL.String())
	if !ok {
		req.RequestURI = ""
		proxyResponse, err := p.client.Do(req)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("proxy couldn't forward request to destination server (%v)", err))
		}
		cachedResponse = &cache.CachedResponse{Resp: proxyResponse}
		p.cache.Set(req.URL.String(), cachedResponse)
		return cachedResponse.Resp, nil
	}
	return cachedResponse.Resp, nil
}

type requestDump []byte

type responseDump []byte

func dump(request *http.Request, response *http.Response) (requestDump, responseDump, error) {
	dumpedResponse, err := httputil.DumpResponse(response, true)
	if err != nil {
		return nil, nil, err
	}
	dumpedRequest, err := httputil.DumpRequest(request, true)
	if err != nil {
		return nil, nil, err
	}
	return dumpedRequest, dumpedResponse, nil
}

func Hsts(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}