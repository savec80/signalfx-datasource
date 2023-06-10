package plugin

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces- only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.

// NewDatasource creates a new datasource instance.
func newDatasource() datasource.ServeOpts {
	// creates a instance manager for your plugin. The function passed
	// into `NewInstanceManger` is called when the instance is created
	// for the first time or when a datasource configuration changed.
	im := datasource.NewInstanceManager(newDataSourceInstance)
	ds := &SignalFxDatasource{
		im: im,
	}

	return datasource.ServeOpts{
		QueryDataHandler:   ds,
		CheckHealthHandler: ds,
	}
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (datasourceInfo *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (ds *SignalFxDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {

	// Unmarshal the json into datasource settings
	if err := json.Unmarshal(req.PluginContext.DataSourceInstanceSettings.JSONData, &ds.settings); err != nil {
		return nil, err
	}

	defer func() {
		if r := recover(); r != nil {
				log.DefaultLogger.Info("Recovered in QueryData", "error", r)
			}
	}()
	log.DefaultLogger.Info("QueryData", "request", req)

	instance, err := ds.im.Get(req.PluginContext)
	if err != nil {
		log.DefaultLogger.Info("Failed getting connection", "error", err)
		return nil, err
	}
	// create response struct
	response := backend.NewQueryDataResponse()

	instSetting, ok := instance.(*instanceSettings)
    if !ok {
        log.DefaultLogger.Info("Failed getting connection")
        return nil, nil
    }
	// authenticate with AWS services
	// if err := ds.authenticate(ctx, req); err != nil {
	// return nil, err
	// }
	// if ds.settings.token != "" {
	// 	log.DefaultLogger.Info("Get token", ds.settings)
	// 	token := ds.settings.token
	// }
	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := ds.query(ctx, instSetting, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}


// type queryModel struct {
// 	Format string `json:"format"`
// 	QueryTxt string `json:"queryTxt"`
// }

func getTypeArray(typ string) interface{} {
    log.DefaultLogger.Debug("getTypeArray", "type", typ)
    switch t := typ; t {
        case "timestamp":
            return []time.Time{}
        case "bigint", "int":
            return []int64{}
        case "smallint":
            return []int16{}
        case "boolean":
            return []bool{}
        case "double", "varint", "decimal":
            return []float64{}
        case "float":
            return []float32{}
        case "tinyint":
            return []int8{}
        default:
            return []string{}
    }
}

func toValue(val interface{}, typ string) interface{} {
    if (val == nil) {
        return nil
    }
    switch t := typ; t {
        case "blob":
            return "Blob"
    }
    switch t := val.(type) {
        case float32, time.Time, string, int64, float64, bool, int16, int8:
            return t
        case gocql.UUID:
            return t.String()
        case int:
            return int64(t)
        case *inf.Dec:
            if s, err := strconv.ParseFloat(t.String(), 64); err == nil {
                return s
            }
            return 0
        case *big.Int:
            if s, err := strconv.ParseFloat(t.String(), 64); err == nil {
                return s
            }
            return 0
        default:
            r, err := json.Marshal(val)
            if (err != nil) {
                log.DefaultLogger.Info("Marsheling failed ", "err", err)
            }
            return string(r)
    }
}


func (ds *SignalFxDatasource) query(ctx context.Context, instance *instanceSettings, dataQuery backend.DataQuery) backend.DataResponse {
	var query queryModel
	var frame *data.Frame
	var apiCall SignalFxApiCall
	response := backend.DataResponse{}

	response.Error = json.Unmarshal(dataQuery.JSON, &query)
	if response.Error != nil {
		return response
	}

	// var v interface{}
	// json.Unmarshal(query.JSON, &v)
	// dt := v.(map[string]interface{})
	// if response.Error != nil {
	//    log.DefaultLogger.Warn("Failed unmarsheling json", "err", response.Error, "json ", string(query.JSON))
	// 	return response
	// }

	frame := data.NewFrame("response")
	token := ds.settings.token
	url := ds.settings.url

	log.DefaultLogger.Debug("queryText found", "querytxt", querytxt, "instance", instance, "token", token, "url", url)

	
	switch apiCall.Path {
	case "/v2/metric":
		apiCall.Method = http.MethodGet
		frame, response.Error = ds.getMetrics(ctx, &apiCall, url, token)
	// case "/v2/suggest/_signalflowsuggest":
	// 	apiCall.Method = http.MethodPost
	// 	frame, response.Error = ds.getSuggestions(ctx, &apiCall)
	}

	frame, response.Error = ds.getDatapoints(ctx, url, token)

	// frame, response.Error = s3List(ctx, ds.s3, &query)
	if response.Error != nil {
		return response
	}
	response.Frames = append(response.Frames, frame)

	return response
}

// func (t *SignalFxDatasource) getMetrics(ctx context.Context, apiCall *SignalFxApiCall) (*datasource.DatasourceResponse, error) {
// 	response := MetricResponse{}
// 	t.makeAPICall(tsdbReq, apiCall, &response)
// 	t.logger.Debug("Unmarshalled API response", "response", response)
// 	values := make([]string, 0)
// 	for _, r := range response.Results {
// 		values = append(values, r.Name)
// 	}
// 	return t.formatAsTable(values), nil
// }

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	var status = backend.HealthStatusOk
	var message = "Data source is working"
	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}




// type instanceSettings struct {
// 	cluster *gocql.ClusterConfig
// 	authenticator *gocql.PasswordAuthenticator
// 	sessions map[string]*gocql.Session
// }



// func (settings *instanceSettings) getSession(hostRef interface{}) (*gocql.Session, error) {
// 	if r := recover(); r != nil {
// 			log.DefaultLogger.Info("Recovered in getSession", "error", r)
// 			var err error= nil
// 			switch x := r.(type) {
// 			case string:
// 					err = errors.New(x)
// 			case error:
// 					err = x
// 			default:
// 					err = errors.New("unknown panic")
// 			}
// 			return nil, err
// 	}
// 	var host string
// 	if hostRef != nil {
// 			host = fmt.Sprintf("%v", hostRef)
// 	}
// 	if val, ok := settings.sessions[host]; ok {
// 			return val, nil
// 	}
// 	if settings.cluster == nil {
// 			if host == "" {
// 					return nil, errors.New("no host supplied for connection")
// 			}
// 			settings.cluster = gocql.NewCluster(host)
// 			log.DefaultLogger.Debug("getSession creating cluster from host", "host", host)
// 			if settings.authenticator != nil {
// 					settings.cluster.Authenticator = *settings.authenticator
// 			}
// 	}
// 	log.DefaultLogger.Debug("getSession", "host", host)
// 	if host == "" {
// 			settings.cluster.HostFilter = nil
// 	} else {
// 			settings.cluster.HostFilter = gocql.WhiteListHostFilter(host)
// 	}
// 	session, err := gocql.NewSession(*settings.cluster)
// 	if err != nil {
// 			log.DefaultLogger.Info("unable to connect to scylla", "err", err, "session", session, "host", host)
// 			return nil, err
// 	}
// 	settings.sessions[host] = session
// 	return session, nil
// }

func NewSignalFxDatasourceInstance(setting backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {

	return &SignalFxDatasource{
		handlers:     make([]SignalFxJob, 0),
		clientMutex:  sync.Mutex{},
		handlerMutex: sync.Mutex{},
		apiClient:    NewSignalFxApiClient(),
	}

// }

// func newDataSourceInstance(setting backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
// 	type editModel struct {
// 			Host string `json:"host"`
// 	}
// 	var hosts editModel
// 	log.DefaultLogger.Debug("newDataSourceInstance", "data", setting.JSONData)
// 	var secureData = setting.DecryptedSecureJSONData
// 	err := json.Unmarshal(setting.JSONData, &hosts)
// 	if err != nil {
// 			log.DefaultLogger.Warn("error marsheling", "err", err)
// 			return nil, err
// 	}
// 	log.DefaultLogger.Info("looking for host", "host", hosts.Host)
// 	var newCluster *gocql.ClusterConfig = nil
// 	var authenticator *gocql.PasswordAuthenticator = nil
// 	password, hasPassword := secureData["password"]
// 	user, hasUser := secureData["user"]
// 	if hasPassword && hasUser {
// 			log.DefaultLogger.Debug("using username and password", "user", user)
// 			authenticator = &gocql.PasswordAuthenticator{
// 					Username: user,
// 					Password: password,
// 			}
// 	}
// 	if hosts.Host != "" {
// 			newCluster = gocql.NewCluster(hosts.Host)
// 			if authenticator != nil {
// 					newCluster.Authenticator = *authenticator
// 			}
// 	}
// return &instanceSettings{
// 	cluster: newCluster,
// 	authenticator: authenticator,
// 	sessions: make(map[string]*gocql.Session),
// }, nil
// }

func (s *instanceSettings) Dispose() {
// Called before creatinga a new instance to allow plugin authors
// to cleanup.
}
