package mallory

import (
	"log"
	"os"
)

type ILoger interface {
	Printf(format string, v ...interface{})
	Print(v ...interface{})
	Println(v ...interface{})
	Fatal(v ...interface{})
	Fatalln(v ...interface{})
}

// global logger
var L ILoger

func init() {
	L = log.New(os.Stdout, "mallory: ", log.Lshortfile|log.LstdFlags)
}
