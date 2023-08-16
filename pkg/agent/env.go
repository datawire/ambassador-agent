package agent

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/datawire/dlib/derror"
	"github.com/datawire/envconfig"
)

type Env struct {
	LogLevel             logrus.Level `env:"LOG_LEVEL,                       parser=log-level,    default=info"`
	AESSnapshotURL       *url.URL     `env:"AES_SNAPSHOT_URL,                parser=absolute-URL, default=http://ambassador-admin:8005/snapshot-external"`
	AESDiagnosticsURL    *url.URL     `env:"AES_DIAGNOSTICS_URL,             parser=absolute-URL, default=http://ambassador-admin:8877/ambassador/v0/diag/?json=true"`
	AESReportDiagnostics bool         `env:"AES_REPORT_DIAGNOSTICS_TO_CLOUD, parser=bool,         default=false"`
	ScoutID              string       `env:"AMBASSADOR_SCOUT_ID,             parser=string,       default="`
	ClusterID            string       `env:"AMBASSADOR_CLUSTER_ID,           parser=string,       defaultFrom=ScoutID"`
	AmbassadorID         string       `env:"AMBASSADOR_ID,                   parser=string,       default=default"`
	AmbassadorAPIKey     string       `env:"CLOUD_CONNECT_TOKEN,             parser=string,       default="`
	ConnAddress          *ConnInfo    `env:"RPC_CONNECTION_ADDRESS,          parser=conn-info,    default="`

	// config map/secret information
	// agent namespace is... the namespace the agent is running in.
	// but more importantly, it's the namespace that the config resource lives in (which is
	// either a ConfigMap or Secret)
	AgentNamespace string `env:"AGENT_NAMESPACE, parser=string, default=ambassador"`

	// Name of the k8s ConfigMap or Secret the CLOUD_CONNECT_TOKEN exists on. We're supporting
	// both Secrets and ConfigMaps here because it is likely in an enterprise cluster, the RBAC
	// for secrets is locked down to Ops folks only, and we want to make it easy for regular ol'
	// engineers to give this whole service catalog thing a go
	AgentConfigResourceName string `env:"AGENT_CONFIG_RESOURCE_NAME, parser=string, default="`

	// Field selector for the k8s resources that the agent watches
	AgentWatchFieldSelector string `env:"AGENT_WATCH_FIELD_SELECTOR, parser=string, default=metadata.namespace!=kube-system"`

	MinReportPeriod         time.Duration `env:"AGENT_REPORTING_PERIOD,          parser=report-period,default="`
	NamespacesToWatch       []string      `env:"NAMESPACES_TO_WATCH,             parser=split-trim,   default="`
	RpcInterceptHeaderKey   string        `env:"RPC_INTERCEPT_HEADER_KEY,        parser=string,       default="`
	RpcInterceptHeaderValue string        `env:"RPC_INTERCEPT_HEADER_VALUE,      parser=string,       default="`

	// ServerHost is the hostname for the gRPC server. Can be empty, in which case it defaults to localhost.
	ServerHost string `env:"SERVER_HOST, parser=string,      default="`

	// ServerPort is the port tha the gRPC server is listening on.
	ServerPort uint16 `env:"SERVER_PORT, parser=port-number, default=8081"`
}

func fieldTypeHandlers() map[reflect.Type]envconfig.FieldTypeHandler {
	fhs := envconfig.DefaultFieldTypeHandlers()
	fp := fhs[reflect.TypeOf("")]
	fp.Parsers["string"] = fp.Parsers["possibly-empty-string"]
	fp = fhs[reflect.TypeOf(true)]
	fp.Parsers["bool"] = fp.Parsers["strconv.ParseBool"]

	fhs[reflect.TypeOf(logrus.Level(0))] = envconfig.FieldTypeHandler{
		Parsers: map[string]func(string) (any, error){
			"log-level": func(str string) (any, error) {
				if str == "" {
					return logrus.InfoLevel, nil
				}
				return logrus.ParseLevel(str)
			},
		},
		Setter: func(dst reflect.Value, src interface{}) { dst.SetUint(uint64(src.(logrus.Level))) },
	}

	fhs[reflect.TypeOf(time.Duration(0))] = envconfig.FieldTypeHandler{
		Parsers: map[string]func(string) (any, error){
			"report-period": func(str string) (any, error) {
				if str == "" {
					return defaultMinReportPeriod, nil
				}
				reportPeriod, err := time.ParseDuration(str)
				if err != nil {
					return 0, err
				}
				return MaxDuration(defaultMinReportPeriod, reportPeriod), nil
			},
		},
		Setter: func(dst reflect.Value, src interface{}) { dst.SetInt(int64(src.(time.Duration))) },
	}

	fhs[reflect.TypeOf([]string{})] = envconfig.FieldTypeHandler{
		Parsers: map[string]func(string) (any, error){
			"split-trim": func(str string) (any, error) { //nolint:unparam // API requirement
				if len(str) == 0 {
					return nil, nil
				}
				ss := strings.Split(str, " ")
				for i, s := range ss {
					ss[i] = strings.TrimSpace(s)
				}
				return ss, nil
			},
		},
		Setter: func(dst reflect.Value, src interface{}) { dst.Set(reflect.ValueOf(src.([]string))) },
	}

	fhs[reflect.TypeOf(&ConnInfo{})] = envconfig.FieldTypeHandler{
		Parsers: map[string]func(string) (any, error){
			"conn-info": func(address string) (any, error) {
				return connInfoFromAddress(address)
			},
		},
		Setter: func(dst reflect.Value, src interface{}) { dst.Set(reflect.ValueOf(src.(*ConnInfo))) },
	}

	fhs[reflect.TypeOf(uint16(0))] = envconfig.FieldTypeHandler{
		Parsers: map[string]func(string) (any, error){
			"port-number": func(str string) (any, error) {
				pn, err := strconv.ParseUint(str, 10, 16)
				return uint16(pn), err
			},
		},
		Setter: func(dst reflect.Value, src interface{}) { dst.SetUint(uint64(src.(uint16))) },
	}
	return fhs
}

func LoadEnv(lookupFunc func(string) (string, bool)) (*Env, error) {
	env := Env{}
	parser, err := envconfig.GenerateParser(reflect.TypeOf(env), fieldTypeHandlers())
	if err != nil {
		panic(err)
	}
	var errs derror.MultiError
	warn, fatal := parser.ParseFromEnv(&env, lookupFunc)
	errs = append(errs, warn...)
	errs = append(errs, fatal...)

	// TODO: the parsing of the following two urls are re-done manually here here since
	// when the deployment specifies a url with characters that need to be escapes like `?`/`=`
	// Kubernetes automatically adds the backslash when applying the manifest. This messes up the parsing
	// since it is expecting a clean URL. This can probably be remedied with the longer term goal of
	// getting rid of the envconfig package from emissary

	snapshotURLEnv, ok := lookupFunc("AES_SNAPSHOT_URL")
	if !ok {
		snapshotURLEnv = "http://ambassador-admin:8005/snapshot-external"
	} else {
		snapshotURLEnv = strings.ReplaceAll(snapshotURLEnv, "\\", "")
	}
	snapshotURL, err := url.Parse(snapshotURLEnv)
	if err != nil {
		errs = append(errs, fmt.Errorf("error parsing AES_SNAPSHOT_URL as a url: %v", err.Error()))
	}
	env.AESSnapshotURL = snapshotURL

	diagnosticsURLEnv, ok := lookupFunc("AES_DIAGNOSTICS_URL")
	if !ok {
		diagnosticsURLEnv = "http://ambassador-admin:8877/ambassador/v0/diag/?json=true"
	} else {
		diagnosticsURLEnv = strings.ReplaceAll(diagnosticsURLEnv, "\\", "")
	}
	diagnosticsURL, err := url.Parse(diagnosticsURLEnv)
	if err != nil {
		errs = append(errs, fmt.Errorf("error parsing AES_DIAGNOSTICS_URL as a url: %v", err.Error()))
	}
	env.AESDiagnosticsURL = diagnosticsURL

	if len(errs) > 0 {
		return nil, errs
	}
	return &env, nil
}
