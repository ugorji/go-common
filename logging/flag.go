package logging

import (
	"flag"
	"time"
)

type Config struct {
	Files         string
	MinLevelStr   string
	MinLevel      Level
	FlushInterval time.Duration
	Async         bool
	BufferSize    int
}

func (p *Config) Flags(flags *flag.FlagSet) {
	// println(">>>>>>>>>>>>>>>>> logging flags:", p)
	flags.StringVar(&p.Files, "log", "<stderr>", "Log file")
	flags.StringVar(&p.MinLevelStr, "loglevel", "", "Log Level Threshold")
	flags.DurationVar(&p.FlushInterval, "logflush", 1*time.Second, "Log Flush Interval")
	flags.BoolVar(&p.Async, "logasync", false, "Log Async (using a serialized channel)")
	flags.IntVar(&p.BufferSize, "logbuf", 64<<10, "Log Buffer Size") //64KB (about 200 lines)
}

func (p *Config) PostParseFlags() {
	if p.MinLevelStr != "" {
		p.MinLevel = ParseLevel(p.MinLevelStr)
	}
	// println(">>>>>>>>>>>>>>>>> logging level:", p, p.MinLevelStr, ",", p.MinLevel)
}
