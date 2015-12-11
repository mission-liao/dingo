package transport

type Option struct {
	// IgnoreReport: stop reporting when executing tasks.
	IR bool
	// OnlyResult: only the last report would be sent: Done or Fail
	OR bool
}

func (me *Option) IgnoreReport() bool { return me.IR }
func (me *Option) OnlyResult() bool   { return me.OR }

func (me *Option) SetIgnoreReport(ignore bool) *Option {
	me.IR = ignore
	return me
}
func (me *Option) SetOnlyResult(only bool) *Option {
	me.OR = only
	return me
}

func NewOption() *Option {
	return &Option{}
}
