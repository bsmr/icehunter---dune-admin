package main

import (
	"fmt"
	"net/http"
)

// @Summary Landsraad overview — latest term, decree catalogue, and task board
// @Tags landsraad
// @Produce json
// @Success 200 {object} landsraadOverview
// @Failure 500 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /api/v1/landsraad [get]
func handleGetLandsraad(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)
	if db == nil {
		jsonErr(w, fmt.Errorf("database not connected"), http.StatusServiceUnavailable)
		return
	}
	ov, err := cmdFetchLandsraad(r.Context(), db)
	if err != nil {
		componentLog("handlers").Error().Err(err).Msg("fetch landsraad failed")
		jsonErr(w, fmt.Errorf("internal error"), http.StatusInternalServerError)
		return
	}
	jsonOK(w, ov)
}
