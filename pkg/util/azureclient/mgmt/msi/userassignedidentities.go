package msi

// Copyright (c) Microsoft Corporation.
// Licensed under the Apache License 2.0.

import (
	"context"

	mgmtmsi "github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/go-autorest/autorest"
)

// UserAssignedIdentitiesClient is a minimal interface for azure UserAssignedIdentitiesClient
type UserAssignedIdentitiesClient interface {
	Get(ctx context.Context, resourceGroupName string, resourceName string) (result mgmtmsi.Identity, err error)
}

type userAssignedIdentitiesClient struct {
	mgmtmsi.UserAssignedIdentitiesClient
}

var _ UserAssignedIdentitiesClient = &userAssignedIdentitiesClient{}

// NewUserAssignedIdentitiesClient creates a new UserAssignedIdentitiesClient
func NewUserAssignedIdentitiesClient(subscriptionID string, authorizer autorest.Authorizer) UserAssignedIdentitiesClient {
	client := mgmtmsi.NewUserAssignedIdentitiesClient(subscriptionID)
	client.Authorizer = authorizer

	return &userAssignedIdentitiesClient{
		UserAssignedIdentitiesClient: client,
	}
}
