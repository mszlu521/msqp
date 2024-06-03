package metrics

import (
	"github.com/arl/statsviz"
	"net/http"
)

// Serve 可视化实时监控 /debug/statsviz
func Serve(add string) error {
	mux := http.NewServeMux()
	if err := statsviz.Register(mux); err != nil {
		return err
	}
	if err := http.ListenAndServe(add, mux); err != nil {
		return err
	}
	return nil
}
