package signalr_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/tacoo/signalr"
)

// This is some testception right here...

type writeFailer struct {
	http.ResponseWriter
	err string
}

func (w writeFailer) Write(p []byte) (int, error) {
	return 0, errors.New(w.err)
}

func catchErr(f http.HandlerFunc, w http.ResponseWriter, r *http.Request) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	f(w, r)
	return nil
}

func TestTestCompleteHandler(t *testing.T) {
	cases := map[string]struct {
		path            string
		header          http.Header
		isWebsocketCall bool
		customWriter    http.ResponseWriter
		customOrigin    string
		exp             string
		wantErr         string
	}{
		"negotiate": {
			path: "/negotiate",
			exp:  `{"ConnectionToken":"hello world","ConnectionId":"1234-ABC","URL":"/signalr","ProtocolVersion":"1337"}`,
		},
		"negotiate failure": {
			path:         "/negotiate",
			customWriter: writeFailer{err: "sample negotiate error"},
			wantErr:      "sample negotiate error",
		},
		"connect": {
			path: "/connect",
			header: http.Header{
				"Upgrade":               []string{"websocket"},
				"Connection":            []string{"upgrade"},
				"Sec-Websocket-Version": []string{"13"},
				"Sec-Websocket-Key":     []string{"blablabla"},
			},
			isWebsocketCall: true,
			exp:             `{"S":1}`,
		},
		"connect failure": {
			path: "/connect",
			header: http.Header{
				"Upgrade":               []string{"websocket"},
				"Connection":            []string{"upgrade"},
				"Sec-Websocket-Version": []string{"13"},
				"Sec-Websocket-Key":     []string{"blablabla"},
			},
			customOrigin:    "blabla",
			isWebsocketCall: true,
			wantErr:         "websocket: 'Origin' header value not allowed",
		},
		"reconnect": {
			path: "/reconnect",
			header: http.Header{
				"Upgrade":               []string{"websocket"},
				"Connection":            []string{"upgrade"},
				"Sec-Websocket-Version": []string{"13"},
				"Sec-Websocket-Key":     []string{"blablabla"},
			},
			isWebsocketCall: true,
			exp:             `{"S":1}`,
		},
		"start": {
			path: "/start",
			exp:  `{"Response":"started"}`,
		},
		"start failure": {
			path:         "/start",
			customWriter: writeFailer{err: "sample start error"},
			wantErr:      "sample start error",
		},
	}

	for id, tc := range cases {
		recorder := httptest.NewRecorder()

		var customErr error
		ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tc.customOrigin != "" {
				r.Header.Set("Origin", tc.customOrigin)
				customErr = catchErr(signalr.TestCompleteHandler, w, r)
			} else if tc.customWriter != nil {
				customErr = catchErr(signalr.TestCompleteHandler, tc.customWriter, r)
			} else if !tc.isWebsocketCall {
				signalr.TestCompleteHandler(recorder, r)
			} else {
				signalr.TestCompleteHandler(w, r)
			}
		}))
		ts.Start()
		c := ts.Client()
		u := ts.URL + tc.path

		var err error
		var act string
		if tc.isWebsocketCall {
			var conn *websocket.Conn
			var p []byte
			u = strings.Replace(u, "http://", "ws://", -1)
			conn, _, err = websocket.DefaultDialer.Dial(u, nil)
			if customErr == nil {
				if err != nil {
					panic(err)
				}
				_, p, err = conn.ReadMessage()
				act = string(p)
			}
		} else {
			var req *http.Request
			req, err = http.NewRequest("GET", u, nil)
			if err != nil {
				panic(err)
			}
			req.Header = tc.header
			_, err = c.Do(req)
			if err != nil {
				panic(err)
			}
			act = recorder.Body.String()
		}

		if tc.wantErr != "" {
			if customErr != nil {
				errMatches(t, id, customErr, tc.wantErr)
			} else {
				errMatches(t, id, err, tc.wantErr)
			}
		} else {
			equals(t, id, tc.exp, act)
			ok(t, id, err)
		}
	}
}
