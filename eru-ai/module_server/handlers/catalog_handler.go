package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func ToolCatalogHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		catalog := tools.GetToolCatalog()
		catalogBytes, err := json.Marshal(catalog)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(catalogBytes)
	}
}
