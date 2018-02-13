package main

import (
	"log"
	"os"
	"runtime"

	logs "github.com/appscode/go/log/golog"
	"github.com/kubedb/apiserver/pkg/cmds"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	if err := cmds.NewRootCmd(Version).Execute(); err != nil {
		log.Fatal(err)
	}
}
