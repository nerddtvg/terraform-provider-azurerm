// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package notificationhub

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-sdk/resource-manager/notificationhubs/2023-09-01/hubs"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

var _ sdk.DataSource = NotificationHubDataSource{}

type NotificationHubDataSource struct{}

type NotificationHubDataSourceModel struct {
	Name              string                   `tfschema:"name"`
	NamespaceName     string                   `tfschema:"namespace_name"`
	ResourceGroupName string                   `tfschema:"resource_group_name"`
	Location          string                   `tfschema:"location"`
	ApnsCredential    []ApnsCredentialModel    `tfschema:"apns_credential"`    // Defined in notification_hub_resource
	BrowserCredential []BrowserCredentialModel `tfschema:"browser_credential"` // Defined in notification_hub_resource
	GcmCredential     []GcmCredentialModel     `tfschema:"gcm_credential"`     // Defined in notification_hub_resource
	Tags              map[string]string        `tfschema:"tags"`
}

func (r NotificationHubDataSource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
		},

		"namespace_name": {
			Type:     pluginsdk.TypeString,
			Required: true,
		},

		"resource_group_name": commonschema.ResourceGroupNameForDataSource(),
	}
}

func (r NotificationHubDataSource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"location": commonschema.LocationComputed(),

		"apns_credential": {
			Type:     pluginsdk.TypeList,
			Computed: true,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"application_mode": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"bundle_id": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"key_id": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					// Team ID (within Apple & the Portal) == "AppID" (within the API)
					"team_id": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"token": {
						Type:      pluginsdk.TypeString,
						Computed:  true,
						Sensitive: true,
					},
				},
			},
		},

		"gcm_credential": {
			Type:     pluginsdk.TypeList,
			Computed: true,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"api_key": {
						Type:      pluginsdk.TypeString,
						Computed:  true,
						Sensitive: true,
					},
				},
			},
		},

		"tags": commonschema.TagsDataSource(),
	}
}

func (r NotificationHubDataSource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return hubs.ValidateNotificationHubID
}

func (r NotificationHubDataSource) ResourceType() string {
	return "azurerm_notification_hub"
}

func (NotificationHubDataSource) ModelObject() interface{} {
	return &NotificationHubDataSourceModel{}
}

func (r NotificationHubDataSource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(5 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient
			subscriptionId := metadata.Client.Account.SubscriptionId

			var config NotificationHubDataSourceModel
			if err := metadata.Decode(&config); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			id := hubs.NewNotificationHubID(subscriptionId, config.ResourceGroupName, config.NamespaceName, config.Name)

			resp, err := client.NotificationHubsGet(ctx, id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return fmt.Errorf("%s was not found", id)
				}

				return fmt.Errorf("retrieving %s: %+v", id, err)
			}

			credentials, err := client.NotificationHubsGetPnsCredentials(ctx, id)
			if err != nil {
				return fmt.Errorf("retrieving credentials for %s: %+v", id, err)
			}

			metadata.SetID(id)

			output := NotificationHubDataSourceModel{
				Name:              id.NotificationHubName,
				NamespaceName:     id.NamespaceName,
				ResourceGroupName: id.ResourceGroupName,
			}

			if credentialsModel := credentials.Model; credentialsModel != nil {
				if props := credentialsModel.Properties; props != nil {
					output.ApnsCredential = flattenNotificationHubsDataSourceAPNSCredentials(props.ApnsCredential)

					output.GcmCredential = flattenNotificationHubsDataSourceGCMCredentials(props.GcmCredential)
				}
			}

			if model := resp.Model; model != nil {
				output.Location = location.NormalizeNilable(&model.Location)

				output.Tags = pointer.From(model.Tags)

				return metadata.Encode(&output)
			}

			return nil
		},
	}
}

func flattenNotificationHubsDataSourceAPNSCredentials(input *hubs.ApnsCredential) []ApnsCredentialModel {
	if input == nil {
		return []ApnsCredentialModel{}
	}

	output := ApnsCredentialModel{}

	if bundleId := input.Properties.AppName; bundleId != nil {
		output.BundleId = pointer.From(bundleId)
	}

	applicationEndpoints := map[string]string{
		apnsProductionEndpoint: apnsProductionName,
		apnsSandboxEndpoint:    apnsSandboxName,
	}
	applicationMode := applicationEndpoints[input.Properties.Endpoint]
	output.ApplicationMode = applicationMode

	if keyId := input.Properties.KeyId; keyId != nil {
		output.KeyId = pointer.From(keyId)
	}

	if teamId := input.Properties.AppId; teamId != nil {
		output.TeamId = pointer.From(teamId)
	}

	if token := input.Properties.Token; token != nil {
		output.Token = pointer.From(token)
	}

	return []ApnsCredentialModel{output}
}

func flattenNotificationHubsDataSourceGCMCredentials(input *hubs.GcmCredential) []GcmCredentialModel {
	if input == nil {
		return []GcmCredentialModel{}
	}

	output := GcmCredentialModel{
		ApiKey: input.Properties.GoogleApiKey,
	}

	return []GcmCredentialModel{output}
}
