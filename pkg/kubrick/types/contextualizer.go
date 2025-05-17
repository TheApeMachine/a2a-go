package types

import (
	"context"
)

type Contextualizer struct {
	ctx    context.Context
	cancel context.CancelFunc
}

type ContextualizerOption func(*Contextualizer)

func NewContextualizer(options ...ContextualizerOption) *Contextualizer {
	ctxt := &Contextualizer{}

	for _, option := range options {
		option(ctxt)
	}

	return ctxt
}

func (ctx *Contextualizer) WithContext(pctx context.Context) {
	ctx.ctx, ctx.cancel = context.WithCancel(pctx)
}

func (ctx *Contextualizer) Context() context.Context {
	return ctx.ctx
}

func (ctx *Contextualizer) Cancel() {
	ctx.cancel()
}

func (ctx *Contextualizer) Done() <-chan struct{} {
	return ctx.ctx.Done()
}

func WithContext(ctx context.Context) ContextualizerOption {
	return func(ctxt *Contextualizer) {
		ctxt.ctx = ctx
	}
}