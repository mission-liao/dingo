package dingo

type Option struct {
	// IgnoreReport: stop reporting when executing tasks.
	IR bool
	// MonitorProgress: monitoring the progress of task execution
	MP bool
}

func (me *Option) IgnoreReport() bool    { return me.IR }
func (me *Option) MonitorProgress() bool { return me.MP }

func (me *Option) SetIgnoreReport(ignore bool) *Option {
	me.IR = ignore
	return me
}
func (me *Option) SetMonitorProgress(only bool) *Option {
	me.MP = only
	return me
}

func NewOption() *Option {
	return &Option{}
}
