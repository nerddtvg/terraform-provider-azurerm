// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package custompollers

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-sdk/resource-manager/notificationhubs/2023-09-01/namespaces"
	"github.com/hashicorp/go-azure-sdk/sdk/client/pollers"
)

var _ pollers.PollerType = &notificationHubNamespacePoller{}

type notificationHubNamespacePoller struct {
	client *namespaces.NamespacesClient
	id     namespaces.NamespaceId
}

var (
	notificationHubNamespacePollerSuccess = pollers.PollResult{
		PollInterval: 5 * time.Second,
		Status:       pollers.PollingStatusSucceeded,
	}
	notificationHubNamespacePollerInProgress = pollers.PollResult{
		HttpResponse: nil,
		PollInterval: 5 * time.Second,
		Status:       pollers.PollingStatusInProgress,
	}
)

func NewNotificationHubNamespacePoller(client *namespaces.NamespacesClient, id namespaces.NamespaceId) *notificationHubNamespacePoller {
	return &notificationHubNamespacePoller{
		client: client,
		id:     id,
	}
}

func (p notificationHubNamespacePoller) Poll(ctx context.Context) (*pollers.PollResult, error) {
	resp, err := p.client.Get(ctx, p.id)
	if err != nil {
		if response.WasNotFound(resp.HttpResponse) {
			return &notificationHubNamespacePollerInProgress, nil
		}

		return nil, fmt.Errorf("retrieving %s: %+v", p.id, err)
	}

	return &notificationHubNamespacePollerSuccess, nil
}
