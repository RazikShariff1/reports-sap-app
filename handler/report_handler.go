package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"

	"gofr.dev/pkg/gofr/container"

	"reports-app/communication"
	"reports-app/entity"
	"reports-app/report"
	"reports-app/store"
)

// maxCustomColumns bounds how many blank fillable columns a PDF report can
// request, keeping each column wide enough to be usable on an A4 page.
const maxCustomColumns = 4

type ReportHandler struct {
	store *store.ReportStore
}

func New(s *store.ReportStore) *ReportHandler {
	return &ReportHandler{store: s}
}

type createReportRequest struct {
	MIDs              []int         `json:"m_ids"`
	ProfessionTypeIDs []int         `json:"profession_type_ids"`
	FileName          string        `json:"file_name"`
	Format            entity.Format `json:"format"`
	AccountID         int           `json:"account_id"`
	CustomColumns     []string      `json:"custom_columns"`
}

// Create validates the filter, inserts a pending report row, and kicks off
// generation in the background so the request returns immediately.
func (h *ReportHandler) Create(ctx *gofr.Context) (any, error) {
	var body createReportRequest
	if err := ctx.Bind(&body); err != nil {
		return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"body"}}
	}

	var missing []string

	if len(body.MIDs) == 0 {
		missing = append(missing, "m_ids")
	}

	if len(body.ProfessionTypeIDs) == 0 {
		missing = append(missing, "profession_type_ids")
	}

	if body.FileName == "" {
		missing = append(missing, "file_name")
	}

	if body.Format == "" {
		missing = append(missing, "format")
	}

	if body.AccountID <= 0 {
		missing = append(missing, "account_id")
	}

	if len(missing) > 0 {
		return nil, gofrHTTP.ErrorMissingParam{Params: missing}
	}

	if body.Format != entity.FormatCSV && body.Format != entity.FormatPDF {
		return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"format"}}
	}

	if len(body.CustomColumns) > maxCustomColumns {
		return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"custom_columns"}}
	}

	params := entity.Params{
		MIDs:              body.MIDs,
		ProfessionTypeIDs: body.ProfessionTypeIDs,
		FileName:          body.FileName,
		Format:            body.Format,
		AccountID:         body.AccountID,
		CustomColumns:     body.CustomColumns,
	}

	id, err := h.store.Create(ctx, ctx.Container, params)
	if err != nil {
		return nil, err
	}

	// ctx.Container outlives the request (ctx.Context is canceled once this handler returns),
	// so the background goroutine must use it directly rather than the per-request *gofr.Context.
	c := ctx.Container
	go h.process(c, id, params)

	return map[string]any{"id": id, "status": entity.StatusPending}, nil
}

func (h *ReportHandler) process(c *container.Container, id int64, params entity.Params) {
	bgCtx := context.Background()

	if err := h.store.MarkProcessing(bgCtx, c, id); err != nil {
		c.Logger.Errorf("report %d: mark processing: %v", id, err)
		return
	}

	outputPath, err := report.Generate(bgCtx, id, params.FileName, params.Format, params.MIDs, params.ProfessionTypeIDs, params.CustomColumns)
	if err != nil {
		if markErr := h.store.MarkFailed(bgCtx, c, id, err.Error()); markErr != nil {
			c.Logger.Errorf("report %d: mark failed: %v", id, markErr)
		}

		return
	}

	if err := h.store.MarkProcessed(bgCtx, c, id, outputPath); err != nil {
		c.Logger.Errorf("report %d: mark processed: %v", id, err)
		return
	}

	pushErr := communication.SendPush(bgCtx, params.AccountID, "Report ready",
		fmt.Sprintf("Your report %q is ready.", params.FileName),
		map[string]string{"report_id": strconv.FormatInt(id, 10), "output_path": outputPath})
	if pushErr != nil {
		c.Logger.Errorf("report %d: send push notification: %v", id, pushErr)
	}
}

// List returns every report requested by the given account_id, most recent first.
func (h *ReportHandler) List(ctx *gofr.Context) (any, error) {
	accountIDStr := ctx.Param("account_id")
	if accountIDStr == "" {
		return nil, gofrHTTP.ErrorMissingParam{Params: []string{"account_id"}}
	}

	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"account_id"}}
	}

	reports, err := h.store.ListByAccountID(ctx, ctx.Container, accountID)
	if err != nil {
		return nil, err
	}

	return reports, nil
}

// Get returns the current status/result of a report.
func (h *ReportHandler) Get(ctx *gofr.Context) (any, error) {
	idStr := ctx.PathParam("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"id"}}
	}

	r, err := h.store.GetByID(ctx, ctx.Container, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, gofrHTTP.ErrorEntityNotFound{Name: "report", Value: idStr}
		}

		return nil, err
	}

	return r, nil
}
