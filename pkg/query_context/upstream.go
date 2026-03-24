package query_context

var finalUpstreamKey = RegKey()

// SetFinalUpstream records the chosen upstream for the current query.
func SetFinalUpstream(ctx *Context, upstream string) {
	if len(upstream) == 0 {
		return
	}
	ctx.StoreValue(finalUpstreamKey, upstream)
}

// GetFinalUpstream returns the chosen upstream for the current query.
func GetFinalUpstream(ctx *Context) (string, bool) {
	v, ok := ctx.GetValue(finalUpstreamKey)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && len(s) > 0
}
