package insights

// Copyright (c) Microsoft Corporation.
// Licensed under the Apache License 2.0.

import (
	"context"

	mgmtinsights "github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/Azure/go-autorest/autorest"
)

// MetricAlertsClient is a minimal interface for azure MetricAlertsClient
type MetricAlertsClient interface {
	Delete(ctx context.Context, resourceGroupName string, ruleName string) (result autorest.Response, err error)
	ListByResourceGroup(ctx context.Context, resourceGroupName string) (result mgmtinsights.MetricAlertResourceCollection, err error)
}

type metricAlertsClient struct {
	mgmtinsights.MetricAlertsClient
}

var _ MetricAlertsClient = &metricAlertsClient{}

// NewMetricAlertsClient creates a new MetricAlertsClient
func NewMetricAlertsClient(subscriptionID string, authorizer autorest.Authorizer) MetricAlertsClient {
	client := mgmtinsights.NewMetricAlertsClient(subscriptionID)
	client.Authorizer = authorizer

	return &metricAlertsClient{
		MetricAlertsClient: client,
	}
}
