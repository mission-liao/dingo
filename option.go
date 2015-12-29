package dingo

type Option struct {
	// IgnoreReport: stop reporting when executing tasks.
	IR bool
	// MonitorProgress: monitoring the progress of task execution
	MP bool
}

func (opt *Option) GetIgnoreReport() bool    { return opt.IR }
func (opt *Option) GetMonitorProgress() bool { return opt.MP }

func (opt *Option) IgnoreReport(ignore bool) *Option {
	opt.IR = ignore
	return opt
}
func (opt *Option) MonitorProgress(only bool) *Option {
	opt.MP = only
	return opt
}

func DefaultOption() *Option {
	return &Option{}
}
