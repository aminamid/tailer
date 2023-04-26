package main

import (
	_ "embed"
	"log"
	"flag"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"
    "gopkg.in/yaml.v3"

	"github.com/nxadm/tail"

	"github.com/aminamid/tailer/chk"
	"github.com/aminamid/tailer/parser"
)

var (
	//go:embed version.txt
	version      string
	showVersion  bool
	skipChk      bool
	cfgFile      string
	cfgInit      string
	exporterAddr string
)
type MoniterFile struct {
  Id string `json:"id"`
  Path string `json:"path"`
  Type string `json:"type"`
}
type Config struct{
  Files []MoniterFile `json:"files"`
}


func main() {
	flag.BoolVar(&showVersion, "v", false, "show version information")
	flag.BoolVar(&skipChk, "q", false, "skip checking configuration")
	flag.StringVar(&exporterAddr, "e", ":10020", "exporter port for prometheus")
	flag.StringVar(&cfgFile, "f", "./tailercfg.yml", "config file")
	flag.StringVar(&cfgInit, "F", "", "configure custom parser")
	flag.Parse()
	if showVersion {
		f1, _ := filepath.Abs(".")
		f2, _ := filepath.Abs("logs")
		fmt.Printf("%s\n%s\n%s\n", version, f1, f2)
		os.Exit(0)
	}
	parser.InitParser(cfgInit)
	chk.ChkYaml(cfgFile)
    cfgBytes,err := os.ReadFile(cfgFile)
	if err != nil {
		log.Fatalf("%v\n",err)
	}
	var cfg Config
    err = yaml.Unmarshal(cfgBytes, &cfg)
    if err != nil {
        log.Fatalf("%s\n",err)
    }
	for _,c := range cfg.Files {
		//fmt.Printf("each cfg type:%s,Id:%s\n",c.Type,c.Id)
		p := parser.NewParser(c.Type,c.Id,c.Path)
		go p.Collector(p.Chan)
		go logWatcher(p)
	}

	parser.MetricsListen(exporterAddr)
}

func logWatcher(p *parser.Parser) {
	var err error
	var t *tail.Tail
	seek := tail.SeekInfo{
		Offset: 0,
		Whence: 2,
		//Whence: 0,
	}

	for {
		t, err = tail.TailFile(p.Path, tail.Config{Location: &seek, ReOpen: true, Follow: true, CompleteLines: true})
		if err != nil {
			fmt.Printf("%v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
	for line := range t.Lines {
		//fmt.Printf("processing: %s\n",line.Text)
		if line.Err != nil {
			fmt.Printf("%v\n", line.Err)
			continue
		}
		notMatch := true
		for i, re := range p.Regxs {
			//fmt.Printf("going to match: %s\n  and         :%s\n",line.Text,p.Restrings[i])
			matches := re.FindStringSubmatch(line.Text)
			if len(matches) < 1 {
				continue
			}
			rec := make(map[string]string)
			rec["id"] = p.Id
			for j, x := range matches {
				rec[p.Keys[i][j]] = x
			}
			var buf bytes.Buffer
			if err = p.TagTmpl[i].Execute(&buf, rec); err != nil {
				log.Fatalf("%#v",err)
			}
			rec["tag"] = buf.String()
			p.Chan <- &parser.Logdata{Data: rec, GaugeNames: p.Values[i]}
			notMatch = false
			break
		}
		if notMatch {
			fmt.Printf("## NOMATCH ## %s\n", line.Text)
			for _,rr := range p.Restrings {
				fmt.Printf(" # NOMATCH ## %s\n",rr)
			}
		}
	}
}
