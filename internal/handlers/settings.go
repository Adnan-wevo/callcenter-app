package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
)

type settingRow struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// GET /api/v1/secure/call-center/settings/categories
func (h *Handlers) ListSettingCategories(c *gin.Context) {
	cats, err := h.settings.Categories(c.Request.Context())
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not load settings categories", nil)
		return
	}
	apires.OK(c, cats)
}

// GET /api/v1/secure/call-center/settings/:category
func (h *Handlers) ListSettingsForCategory(c *gin.Context) {
	rows, err := h.settings.ListCategory(c.Request.Context(), c.Param("category"))
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not load settings", nil)
		return
	}
	out := make([]settingRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, settingRow{Key: row.Key, Value: row.ValueString(), Description: row.Description.String})
	}
	apires.OK(c, out)
}

type updateSettingsBody struct {
	Values map[string]string `json:"values" binding:"required"`
}

// PUT /api/v1/secure/call-center/settings/:category
func (h *Handlers) UpdateSettingsForCategory(c *gin.Context) {
	var body updateSettingsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "values is required", nil)
		return
	}
	if err := h.settings.SetCategory(c.Request.Context(), c.Param("category"), body.Values); err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not save settings", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "updated"})
}
