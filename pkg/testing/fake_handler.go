package testing

//
//import (
//	"io/ioutil"
//	"net/http"
//	"net/url"
//	"reflect"
//	"sync"
//	"testing"
//)
//
//// TestInterface is a simple interface providing Errorf, to make injection for
//// testing easier.
//type TestInterface interface {
//	Errorf(format string, args ...interface{})
//	Logf(format string, args ...interface{})
//}
//
//// LogInterface is a simple interface to allow injection of Logf to report serving errors.
//type LogInterface interface {
//	Logf(format string, args ...interface{})
//}
//
//type FakeHandler struct {
//	RequestGot    *http.Request
//	RequestBody   string
//	RequestMethod string
//	StatusCode    int
//	ResponseBody  string
//	// For logging - you can use a *testing.T
//	// This will keep log messages associated with the test.
//	T LogInterface
//
//	// Make it safe for concurrency
//	lock           sync.Mutex
//	requestCount   int
//	hasBeenChecked bool
//
//	SkipRequestFn func(verb string, url url.URL) bool
//}
//
//func NewFakeHandler(statusCode int, responseBody string, reqMethod string, t *testing.T) *FakeHandler {
//	return &FakeHandler{
//		RequestGot:     nil,
//		RequestBody:    "",
//		RequestMethod:  reqMethod,
//		StatusCode:     statusCode,
//		ResponseBody:   responseBody,
//		T:              t,
//		lock:           sync.Mutex{},
//		requestCount:   0,
//		hasBeenChecked: false,
//		SkipRequestFn:  nil,
//	}
//}
//
//func (f *FakeHandler) SetResponseBody(responseBody string) {
//	f.lock.Lock()
//	defer f.lock.Unlock()
//	f.ResponseBody = responseBody
//}
//
//func (f *FakeHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
//	f.lock.Lock()
//	defer f.lock.Unlock()
//
//	f.requestCount++
//	if f.hasBeenChecked {
//		panic("got request after having been validated")
//	}
//
//	f.RequestGot = request
//	response.Header().Set("Content-Type", "application/json")
//	response.WriteHeader(f.StatusCode)
//	_, _ = response.Write([]byte(f.ResponseBody))
//
//	bodyReceived, err := ioutil.ReadAll(request.Body)
//	if err != nil && f.T != nil {
//		f.T.Logf("Received read error: %v", err)
//	}
//	f.RequestBody = string(bodyReceived)
//	if f.T != nil && f.RequestBody != "" {
//		f.T.Logf("request body: %s", f.RequestBody)
//	}
//}
//
//func (f *FakeHandler) ValidateRequestCount(t TestInterface, count int) bool {
//	ok := true
//	f.lock.Lock()
//	defer f.lock.Unlock()
//	if f.requestCount != count {
//		ok = false
//		t.Errorf("Expected %d call, but got %d. Only the last call is recorded and checked.", count, f.requestCount)
//	}
//	f.hasBeenChecked = true
//	return ok
//}
//
//// ValidateRequest verifies that FakeHandler received a request with expected path, method, and body.
//func (f *FakeHandler) ValidateRequest(t TestInterface, expectedPath, expectedMethod string, body *string) {
//	f.lock.Lock()
//	defer f.lock.Unlock()
//	expectURL, err := url.Parse(expectedPath)
//	if err != nil {
//		t.Errorf("couldn't parse %v as a URL.", expectedPath)
//		return
//	}
//	if f.RequestGot == nil {
//		t.Errorf("unexpected nil request received for %s", expectedPath)
//		return
//	}
//	if f.RequestGot.URL.Path != expectURL.Path {
//		t.Errorf("unexpected request path for request %#v, Expected: %q, Got: %q", f.RequestGot, expectURL.Path, f.RequestGot.URL.Path)
//	}
//	if e, g := expectURL.Query(), f.RequestGot.URL.Query(); !reflect.DeepEqual(e, g) {
//		t.Errorf("unexpected query for request %#v, Expected: %v, Got: %v", f.RequestGot, e, g)
//	}
//	if f.RequestGot.Method != expectedMethod {
//		t.Errorf("unexpected method. Expected: %q, Got: %q", expectedMethod, f.RequestGot.Method)
//	}
//	if body != nil {
//		if *body != f.RequestBody {
//			t.Errorf("body got did not matched expected, Expected:\n%s\n Got:\n%s", f.RequestBody, *body)
//		}
//	}
//}
