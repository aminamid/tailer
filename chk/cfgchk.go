package chk

import (
	_ "embed"

    "log"
    "os"

    "cuelang.org/go/cue"
    "cuelang.org/go/cue/cuecontext"
    "cuelang.org/go/encoding/yaml"
	"cuelang.org/go/cue/errors"
)
var (
        //go:embed schema.cue
        schema string
)

//type MoniterFile struct {
//	Id string `json:"id"`
//	Path string `json:"path"`
//	Type string `json:"type"` 
//}
//type Config struct{
//	Files []MoniterFile `json:"files"`
//}
func ChkYaml(configfile string) {
	ctx := cuecontext.New()
	schemaValue := ctx.CompileString(schema, cue.Filename("schema"))

	ymlByte, err := os.ReadFile(configfile)
    if err != nil {
        log.Fatalf("Error reading YAML file:", err)
    }

     ymlFile, err := yaml.Extract(configfile, ymlByte)
     if err != nil {
         log.Fatalf("Error decoding YAML:", err)
     }
     ymlValue := ctx.BuildFile(ymlFile,cue.Filename(configfile),cue.Scope(schemaValue))

	
     newValue := schemaValue.Unify(ymlValue)
	 err = newValue.Validate()
	 if err != nil {
		log.Fatalf("%s\n", errors.Details(err,nil))
	 }
}

