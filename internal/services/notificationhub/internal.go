// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package notificationhub

import (
	"context"
	"errors"

	"github.com/hashicorp/go-azure-sdk/resource-manager/notificationhubs/2023-09-01/hubs"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
)

func expandNotificationHubAuthorizationRuleRights(manage bool, send bool, listen bool) []hubs.AccessRights {
	rights := make([]hubs.AccessRights, 0)

	if manage {
		rights = append(rights, hubs.AccessRightsManage)
	}

	if send {
		rights = append(rights, hubs.AccessRightsSend)
	}

	if listen {
		rights = append(rights, hubs.AccessRightsListen)
	}

	return rights
}

func flattenNotificationHubAuthorizationRuleRights(input *[]hubs.AccessRights) (manage bool, send bool, listen bool) {
	if input == nil {
		return
	}

	for _, right := range *input {
		switch right {
		case hubs.AccessRightsManage:
			manage = true
			continue
		case hubs.AccessRightsSend:
			send = true
			continue
		case hubs.AccessRightsListen:
			listen = true
			continue
		}
	}

	return manage, send, listen
}

func authorizationRuleCustomizeDiff(ctx context.Context, metadata sdk.ResourceMetaData) error {
	listen, hasListen := metadata.ResourceDiff.GetOk("listen")
	send, hasSend := metadata.ResourceDiff.GetOk("send")
	manage, hasManage := metadata.ResourceDiff.GetOk("manage")

	if !hasListen && !hasSend && !hasManage {
		return errors.New("one of the `listen`, `send` or `manage` properties needs to be set")
	}

	if manage.(bool) && (!listen.(bool) || !send.(bool)) {
		return errors.New("if `manage` is set both `listen` and `send` must be set to true too")
	}

	return nil
}
