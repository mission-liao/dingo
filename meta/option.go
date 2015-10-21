package meta

type Option struct {
	IgnoreReport_ bool `json:"IgnoreReport"`
}

func (me *Option) IgnoreReport(yes bool) *Option {
	me.IgnoreReport_ = yes
	return me
}
