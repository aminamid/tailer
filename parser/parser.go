package parser

import (
    _ "embed"
	"strings"
	"strconv"
	"sync"
	"os"
	"regexp"
	"fmt"
	"log"
	"text/template"
    "gopkg.in/yaml.v3"
    "net/http"
	"github.com/VictoriaMetrics/metrics"

)

var (
    metricsMutex sync.Mutex
	mu sync.RWMutex
	sfcmCfg interface{}
	cfgPath string

	//go:embed parser.yml
	cfgDefault []byte
	cfg interface{}
)
type Logdata struct {
    Data map[string]string
    GaugeNames []string
}

func InitParser(cfgstr string) {
	var cfgBytes []byte
	if len(cfgstr) < 1 {
		cfgBytes = cfgDefault
	} else {
		var err error
		cfgBytes,err = os.ReadFile(cfgstr)
		if err != nil {
			log.Fatalf("%s\n",err)
		}
	}
	err := yaml.Unmarshal(cfgBytes, &cfg)
	if err != nil {
		log.Fatalf("%s\n",err)
	}
	//fmt.Printf("Loaded initcfg: %#v\n", cfg)
}
func loadJson(jsonObj *interface{},inputPath string) error {
    byteArray, _ := os.ReadFile(inputPath)
    return yaml.Unmarshal(byteArray, &jsonObj)
}

func saveJson(jsonObj interface{}, outputPath string) error {
    file, _ := os.Create(outputPath)
    defer file.Close()
    enc := yaml.NewEncoder(file)
    return enc.Encode(jsonObj)
}

type Parser struct {
	Id	string
	Collector func(chan *Logdata)
	Path       string
	Restrings []string
	Regxs      []*regexp.Regexp
	TagTmpl   []*template.Template
	Keys     [][]string
	Values	[][]string
	Chan    chan *Logdata
}

func newParser(collector string,id string,path string) *Parser {
	p := Parser{
		Id: id,
		Path:      path,
		Collector: Collectors[collector], 
		Restrings: make([]string,0,10),
		Regxs:     make([]*regexp.Regexp,0, 10),
		TagTmpl:    make([]*template.Template,0,10),
		Keys:    make([][]string,0,10),
		Values: make([][]string,0,10),
		Chan: make(chan *Logdata,20),
	}
	
	return &p
}
func NewParser(collector string,id string,path string) *Parser {
	p := newParser(collector, id,path)
	if _,ok := cfg.(map[string]interface{})[collector]; !ok {
		log.Fatalf("ERR Type Notfound %s\n",collector)
	}
	for _,c := range cfg.(map[string]interface{})[collector].([]interface{}) {
		x := c.(map[string]interface{})
		//fmt.Printf("  cfg[%s]: regx: %s\n",collector,x["regx"].(string))
		l := make([]string,len(x["list"].([]interface{})))
		for i,li := range x["list"].([]interface{}) {
			l[i] = li.(string)
		}
		p.AddRegx(x["regx"].(string), x["tmpl"].(string), l)
	}
	//fmt.Printf("p.Restrings=%v\n",p.Restrings)
	return p
}



func (p *Parser) AddRegx(s string, templstr string, values []string) {
	p.Restrings = append(p.Restrings, s)
	p.Regxs = append(p.Regxs, regexp.MustCompile(s))
	tmpl, err := template.New(fmt.Sprintf("%s%d",p.Id,len(p.TagTmpl))).Parse(templstr)
	if err != nil {
		log.Fatalf("%#v",err)
	}
	p.TagTmpl = append(p.TagTmpl, tmpl)
	p.Keys = append(p.Keys, (*p.Regxs[len(p.Regxs)-1]).SubexpNames())
	p.Values = append(p.Values, values)
}
var Collectors = map[string]func (chan *Logdata) {
	"mxlog": CountCollector,
	"mxstat": GaugeCollector,
	"moslog": CountCollector,
	"mosstat": GaugeCollector,
	"rglog": RgCollector,
}
func RgCollector(chRec chan *Logdata) {
    for logData := range chRec {
		if _,isStat := logData.Data["statname"]; isStat {
			strslots := strings.Split(logData.Data["slots"],",")
			slots := make([]float64,0,30)
			var slots_nonzero_values_count float64 = 0
			var slots_value_sum float64 = 0
			var slots_max_value float64 = 0
			for _,s := range strslots {
				f, err := strconv.ParseFloat(s, 64)
	            if err != nil {
	                fmt.Printf(`Fail to convert "%s": %#v\n`, s, err)
	                continue
	            }
				slots = append(slots, f)
				if f != 0 {
					slots_nonzero_values_count += 1
					slots_value_sum += f
				}
				if f > slots_max_value {
					slots_max_value = f
				}
			}
            metricsMutex.Lock()
            metrics.GetOrCreateGauge(fmt.Sprintf(logData.Data["tag"],"slots_count"), func() float64 { return float64(len(slots)) })
            metrics.GetOrCreateGauge(fmt.Sprintf(logData.Data["tag"],"slots_nonzero_values_count"), func() float64 { return slots_nonzero_values_count })
            metrics.GetOrCreateGauge(fmt.Sprintf(logData.Data["tag"],"slots_value_sum"), func() float64 { return slots_value_sum })
            metrics.GetOrCreateGauge(fmt.Sprintf(logData.Data["tag"],"slots_max_value"), func() float64 { return slots_max_value })
            metricsMutex.Unlock()
            fmt.Printf("%s %f\n",fmt.Sprintf(logData.Data["tag"],"slots_count"),float64(len(slots)))
            fmt.Printf("%s %f\n",fmt.Sprintf(logData.Data["tag"],"slots_nonzero_values_count"),slots_nonzero_values_count)
            fmt.Printf("%s %f\n",fmt.Sprintf(logData.Data["tag"],"slots_value_sum"),slots_value_sum)
            fmt.Printf("%s %f\n",fmt.Sprintf(logData.Data["tag"],"slots_max_value"),slots_max_value)
		} else {
        	metricsMutex.Lock()
			//fmt.Printf("rgcollect logData:  %#v\nrgcollect logData.Data %#v\n", logData, logData.Data)
        	c := metrics.GetOrCreateCounter(logData.Data["tag"])
        	c.Inc()
        	metricsMutex.Unlock()
        	fmt.Printf("%s %d\n",logData.Data["tag"], c.Get())
		}	
    }
}
func CountCollector(chRec chan *Logdata) {
    for logData := range chRec {
        metricsMutex.Lock()
		//fmt.Printf("Countcollect logData:  %#v\nCountcollect logData.Data %#v\n", logData, logData.Data)
        c := metrics.GetOrCreateCounter(logData.Data["tag"])
        c.Inc()
        metricsMutex.Unlock()
        fmt.Printf("%s %d\n",logData.Data["tag"], c.Get())
    }
}
func GaugeCollector(chRec chan *Logdata) {
    for logData := range chRec {
        for _,key := range logData.GaugeNames {
            f, err := strconv.ParseFloat(logData.Data[key], 64)
            if err != nil {
                fmt.Printf(`Fail to convert "%s": %#v\n`, logData.Data[key], err)
                continue
            }
            metricsMutex.Lock()
            metrics.GetOrCreateGauge(fmt.Sprintf(logData.Data["tag"],key), func() float64 { return f })
            metricsMutex.Unlock()
            fmt.Printf("%s %f\n",fmt.Sprintf(logData.Data["tag"],key),f)
        }
    }
}

func MetricsListen(listenAddr string) {
    http.HandleFunc("/metrics", metricsHandler)
    http.ListenAndServe(listenAddr, nil)
}

func metricsHandler(w http.ResponseWriter, _ *http.Request) {
    metricsMutex.Lock()
    defer metricsMutex.Unlock()

    metrics.WritePrometheus(w, false)
    //metrics.UnregisterAllMetrics()
}

