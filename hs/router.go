package hs

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
)

type Middleware func(http.Handler) http.Handler

// 路由分组
type Group struct {
	prefix      string
	middlewares []Middleware
	mux         *http.ServeMux
}

func NewGroup(prefix string, mux *http.ServeMux, middlewares ...Middleware) *Group {
	return &Group{
		prefix:      prefix,
		mux:         mux,
		middlewares: middlewares,
	}
}

func (g *Group) Handle(pattern string, handler http.Handler) {
	ps := strings.Split(pattern, " ")
	if len(ps) != 2 {
		panic("pattern must be in the format of 'METHOD PATH'")
	}
	fullPattern := ps[0] + " " + filepath.Join(g.prefix, ps[1])

	h := handler
	for i := len(g.middlewares) - 1; i >= 0; i-- {
		h = g.middlewares[i](h)
	}

	// 打印路由信息
	slog.Info("register route: " + fullPattern)
	g.mux.Handle(fullPattern, h)
}

func (g *Group) HandleFunc(pattern string, handlerFunc http.HandlerFunc) {
	g.Handle(pattern, handlerFunc)
}
