// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package custompollers

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-sdk/resource-manager/notificationhubs/2023-09-01/hubs"
	"github.com/hashicorp/go-azure-sdk/sdk/client/pollers"
)

var _ pollers.PollerType = &notificationHubPoller{}

type notificationHubPoller struct {
	client *hubs.HubsClient
	id     hubs.NotificationHubId
}

var (
	NotificationHubPollerSuccess = pollers.PollResult{
		PollInterval: 5 * time.Second,
		Status:       pollers.PollingStatusSucceeded,
	}
	NotificationHubPollerInProgress = pollers.PollResult{
		HttpResponse: nil,
		PollInterval: 5 * time.Second,
		Status:       pollers.PollingStatusInProgress,
	}
)

func NewNotificationHubPoller(client *hubs.HubsClient, id hubs.NotificationHubId) *notificationHubPoller {
	return &notificationHubPoller{
		client: client,
		id:     id,
	}
}

func (p notificationHubPoller) Poll(ctx context.Context) (*pollers.PollResult, error) {
	resp, err := p.client.NotificationHubsGet(ctx, p.id)
	if err != nil {
		if response.WasNotFound(resp.HttpResponse) {
			return &NotificationHubPollerInProgress, nil
		}

		return nil, fmt.Errorf("retrieving %s: %+v", p.id, err)
	}

	return &NotificationHubPollerSuccess, nil
}
