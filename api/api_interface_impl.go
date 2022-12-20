//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --config=types.cfg.yaml ../docs/openapi.yaml
//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --config=server.cfg.yaml ../docs/openapi.yaml
//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --config=client.cfg.yaml ../docs/openapi.yaml

package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/0xERR0R/blocky/log"
	"github.com/0xERR0R/blocky/model"
	"github.com/0xERR0R/blocky/util"
	"github.com/go-chi/chi/v5"
	"github.com/miekg/dns"
)

// BlockingStatus represents the current blocking status
type BlockingStatus struct {
	// True if blocking is enabled
	Enabled bool
	// Disabled group names
	DisabledGroups []string
	// If blocking is temporary disabled: amount of seconds until blocking will be enabled
	AutoEnableInSec int
}

// BlockingControl interface to control the blocking status
type BlockingControl interface {
	EnableBlocking()
	DisableBlocking(duration time.Duration, disableGroups []string) error
	BlockingStatus() BlockingStatus
}

// ListRefresher interface to control the list refresh
type ListRefresher interface {
	RefreshLists() error
}

type Querier interface {
	Query(question string, qType dns.Type) (*model.Response, error)
}

func RegisterOpenApiEndpoints(router chi.Router, impl StrictServerInterface) {
	HandlerFromMuxWithBaseURL(NewStrictHandler(impl, nil), router, "/api")
}

type OpenApiInterfaceImpl struct {
	control   BlockingControl
	querier   Querier
	refresher ListRefresher
}

func NewOpenApiInterfaceImpl(control BlockingControl, querier Querier, refresher ListRefresher) *OpenApiInterfaceImpl {
	return &OpenApiInterfaceImpl{
		control:   control,
		querier:   querier,
		refresher: refresher,
	}
}

func (i *OpenApiInterfaceImpl) GetBlockingDisable(ctx context.Context, request GetBlockingDisableRequestObject) (GetBlockingDisableResponseObject, error) {
	var (
		duration time.Duration
		groups   []string
		err      error
	)

	if request.Params.Duration != nil {
		duration, err = time.ParseDuration(*request.Params.Duration)
		if err != nil {
			return GetBlockingDisable400TextResponse(log.EscapeInput(err.Error())), nil
		}
	}

	if request.Params.Groups != nil {
		groups = strings.Split(*request.Params.Groups, ",")
	}

	err = i.control.DisableBlocking(duration, groups)

	if err != nil {
		return GetBlockingDisable400TextResponse(log.EscapeInput(err.Error())), nil
	}

	return GetBlockingDisable200Response{}, nil
}

func (i *OpenApiInterfaceImpl) GetBlockingEnable(ctx context.Context, request GetBlockingEnableRequestObject) (GetBlockingEnableResponseObject, error) {
	i.control.EnableBlocking()
	return GetBlockingEnable200Response{}, nil
}

func (i *OpenApiInterfaceImpl) GetBlockingStatus(ctx context.Context, request GetBlockingStatusRequestObject) (GetBlockingStatusResponseObject, error) {
	blStatus := i.control.BlockingStatus()

	result := ApiBlockingStatus{
		Enabled: blStatus.Enabled,
	}

	if blStatus.AutoEnableInSec > 0 {
		result.AutoEnableInSec = &blStatus.AutoEnableInSec
	}

	if len(blStatus.DisabledGroups) > 0 {
		result.DisabledGroups = &blStatus.DisabledGroups
	}

	return GetBlockingStatus200JSONResponse(result), nil
}

func (i *OpenApiInterfaceImpl) PostListsRefresh(ctx context.Context, request PostListsRefreshRequestObject) (PostListsRefreshResponseObject, error) {
	err := i.refresher.RefreshLists()

	if err != nil {
		return PostListsRefresh500TextResponse(log.EscapeInput(err.Error())), nil
	}

	return PostListsRefresh200Response{}, nil
}

func (i *OpenApiInterfaceImpl) PostQuery(ctx context.Context, request PostQueryRequestObject) (PostQueryResponseObject, error) {
	qType := dns.Type(dns.StringToType[request.Body.Type])
	if qType == dns.Type(dns.TypeNone) {
		return PostQuery400TextResponse(fmt.Sprintf("unknown query type '%s'", request.Body.Type)), nil
	}

	resp, err := i.querier.Query(dns.Fqdn(request.Body.Query), qType)

	if err != nil {
		return nil, err
	}

	return PostQuery200JSONResponse(ApiQueryResult{
		Reason:       resp.Reason,
		ResponseType: resp.RType.String(),
		Response:     util.AnswerToString(resp.Res.Answer),
		ReturnCode:   dns.RcodeToString[resp.Res.Rcode],
	}), nil
}
