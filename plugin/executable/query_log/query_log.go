package query_log

import (
	"context"
	"time"

	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

const PluginType = "query_log"

func init() {
	sequence.MustRegExecQuickSetup(PluginType, QuickSetup)
}

var _ sequence.RecursiveExecutable = (*Logger)(nil)

type Logger struct {
	l   *zap.Logger
	msg string
}

// QuickSetup format: [msg_title]
func QuickSetup(bq sequence.BQ, s string) (any, error) {
	if len(s) == 0 {
		s = "query log"
	}
	return &Logger{l: bq.L(), msg: s}, nil
}

func (l *Logger) Exec(ctx context.Context, qCtx *query_context.Context, next sequence.ChainWalker) error {
	err := next.ExecNext(ctx, qCtx)

	fields := make([]zap.Field, 0, 8)
	if client := qCtx.ServerMeta.ClientAddr; client.IsValid() {
		fields = append(fields, zap.String("client_ip", client.Unmap().String()))
	}

	q := qCtx.QQuestion()
	fields = append(fields,
		zap.String("qname", q.Name),
		zap.Uint16("qtype", q.Qtype),
		zap.Uint16("qclass", q.Qclass),
		zap.Duration("elapsed", time.Since(qCtx.StartTime())),
	)

	if upstream, ok := query_context.GetFinalUpstream(qCtx); ok {
		fields = append(fields, zap.String("upstream", upstream))
	}

	if r := qCtx.R(); r != nil {
		fields = append(fields,
			zap.Int("rcode", r.Rcode),
			zap.Strings("answers", answersToStrings(r.Answer)),
		)
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
	}

	l.l.Info(l.msg, fields...)
	return err
}

func answersToStrings(rrs []dns.RR) []string {
	if len(rrs) == 0 {
		return nil
	}
	ss := make([]string, 0, len(rrs))
	for _, rr := range rrs {
		ss = append(ss, rr.String())
	}
	return ss
}
