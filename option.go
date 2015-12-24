package dingo

type Option struct {
	// IgnoreReport: stop reporting when executing tasks.
	IR bool
	// MonitorProgress: monitoring the progress of task execution
	MP bool
}

func (opt *Option) IgnoreReport() bool    { return opt.IR }
func (opt *Option) MonitorProgress() bool { return opt.MP }

func (opt *Option) SetIgnoreReport(ignore bool) *Option {
	opt.IR = ignore
	return opt
}
func (opt *Option) SetMonitorProgress(only bool) *Option {
	opt.MP = only
	return opt
}

func NewOption() *Option {
	return &Option{}
}
