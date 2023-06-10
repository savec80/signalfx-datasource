package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/hashicorp/go-plugin"
	"github.com/signalfx/signalfx-go/signalflow"
)

type SignalFxDatasource struct {
	im instancemgmt.InstanceManager

	backend.CallResourceHandler
	plugin.NetRPCUnsupportedPlugin
	handlers     []SignalFxJob
	client       *signalflow.Client
	handlerMutex sync.Mutex
	clientMutex  sync.Mutex
	url          string
	token        string
	apiClient    *SignalFxApiClient
	settings     struct {
		url   string `json:"bucket" binding:"Required"`
		token string `json:"region" binding:"Required"`
	}
}

type queryModel struct {
	RefID         string        `json:"refId"`
	Program       string        `json:"program"`
	StartTime     time.Time     `json:"-"`
	StopTime      time.Time     `json:"-"`
	Interval      time.Duration `json:"-"`
	Alias         string        `json:"alias"`
	MaxDelay      int64         `json:"maxDelay"`
	MinResolution int64         `json:"minResolution"`
}

type instanceSettings struct {
	httpClient *http.Client
}

