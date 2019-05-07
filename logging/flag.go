package logging

import (
	"flag"
	"time"
)

type flagLevel Level

func (v *flagLevel) String() string {
	if v == nil {
		return ""
	}
	return level2s[Level(*v)]
}

func (v *flagLevel) Set(s string) error {
	*v = flagLevel(level4s[s])
	return nil
}

type Flags struct {
	Files string
	Config
}

func (p *Flags) Flags(flags *flag.FlagSet) {
	// println(">>>>>>>>>>>>>>>>> logging flags:", p)
	flags.StringVar(&p.Files, "log", "<stderr>", "Log file")
	// flags.StringVar(&p.MinLevelStr, "loglevel", "", "Log Level Threshold")
	flags.DurationVar(&p.FlushInterval, "logflush", 5*time.Second, "Log Flush Interval")
	// flags.BoolVar(&p.Async, "logasync", false, "Log Async (using a serialized channel)")
	flags.IntVar(&p.BufferSize, "logbuf", 32<<10, "Log Buffer Size, up to MaxUint16 (64K)") // 32KB (about 100 lines)
	flags.Var((*flagLevel)(&p.MinLevel), "loglevel", "Log Level Threshold")
	flags.Var((*flagLevel)(&p.PopulatePCLevel), "loglevelpc", "Populate PC  Level Threshold")
}

// func (p *Config) PostParseFlags() {
// 	if p.MinLevelStr != "" {
// 		p.MinLevel = ParseLevel(p.MinLevelStr)
// 	}
// 	// println(">>>>>>>>>>>>>>>>> logging level:", p, p.MinLevelStr, ",", p.MinLevel)
// }
