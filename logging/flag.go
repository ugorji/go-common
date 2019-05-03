package logging

import (
	"flag"
	"time"
)

type configLevel Level

func (v *configLevel) String() string {
	if v == nil {
		return ""
	}
	return level2s[Level(*v)]
}

func (v *configLevel) Set(s string) error {
	*v = configLevel(level4s[s])
	return nil
}

type Config struct {
	Files         string
	FlushInterval time.Duration
	BufferSize    int
	MinLevel      Level
}

func (p *Config) Flags(flags *flag.FlagSet) {
	// println(">>>>>>>>>>>>>>>>> logging flags:", p)
	flags.StringVar(&p.Files, "log", "<stderr>", "Log file")
	// flags.StringVar(&p.MinLevelStr, "loglevel", "", "Log Level Threshold")
	flags.DurationVar(&p.FlushInterval, "logflush", 1*time.Second, "Log Flush Interval")
	// flags.BoolVar(&p.Async, "logasync", false, "Log Async (using a serialized channel)")
	flags.IntVar(&p.BufferSize, "logbuf", 64<<10, "Log Buffer Size") //64KB (about 200 lines)
	flags.Var((*configLevel)(&p.MinLevel), "loglevel", "Log Level Threshold")
}

// func (p *Config) PostParseFlags() {
// 	if p.MinLevelStr != "" {
// 		p.MinLevel = ParseLevel(p.MinLevelStr)
// 	}
// 	// println(">>>>>>>>>>>>>>>>> logging level:", p, p.MinLevelStr, ",", p.MinLevel)
// }
