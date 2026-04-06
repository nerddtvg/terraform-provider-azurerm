// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package notificationhub

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-sdk/resource-manager/notificationhubs/2023-09-01/hubs"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/locks"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/notificationhub/migration"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

var _ sdk.ResourceWithUpdate = NotificationHubAuthorizationRuleResource{}

type NotificationHubAuthorizationRuleResource struct{}

type NotificationHubAuthorizationRuleResourceModel struct {
	Name                      string `tfschema:"name"`
	NotificationHubName       string `tfschema:"notification_hub_name"`
	NamespaceName             string `tfschema:"namespace_name"`
	ResourceGroupName         string `tfschema:"resource_group_name"`
	Manage                    bool   `tfschema:"manage"`
	Send                      bool   `tfschema:"send"`
	Listen                    bool   `tfschema:"listen"`
	PrimaryAccssKey           string `tfschema:"primary_access_key"`
	SecondaryAccessKey        string `tfschema:"secondary_access_key"`
	PrimaryConnectionString   string `tfschema:"primary_connection_string"`
	SecondaryConnectionString string `tfschema:"secondary_connection_string"`
}

func (r NotificationHubAuthorizationRuleResource) StateUpgraders() sdk.StateUpgradeData {
	return sdk.StateUpgradeData{
		SchemaVersion: 1, // This field references the version which the state migration updates the schema to i.e. v0 -> v1
		Upgraders: map[int]pluginsdk.StateUpgrade{
			0: migration.NotificationHubAuthorizationRuleResourceV0ToV1{},
		},
	}
}

func (r NotificationHubAuthorizationRuleResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

	id := hubs.NewNotificationHubAuthorizationRuleID(subscriptionId, d.Get("resource_group_name").(string), d.Get("namespace_name").(string), d.Get("notification_hub_name").(string), d.Get("name").(string))
	if d.IsNewResource() {
		if !meta.(*clients.Client).Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
			existing, err := client.NotificationHubsGetAuthorizationRule(ctx, id)
			if err != nil {
				if !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
			}

			if !response.WasNotFound(existing.HttpResponse) {
				return tf.ImportAsExistsError("azurerm_notification_hub_authorization_rule", id.ID())
			}
		}
	}
		"notification_hub_name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"namespace_name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"resource_group_name": commonschema.ResourceGroupName(),

		"manage": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},

		"send": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},

		"listen": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},
	}
}

func (r NotificationHubAuthorizationRuleResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"primary_access_key": {
			Type:      pluginsdk.TypeString,
			Computed:  true,
			Sensitive: true,
		},

		"secondary_access_key": {
			Type:      pluginsdk.TypeString,
			Computed:  true,
			Sensitive: true,
		},

		"primary_connection_string": {
			Type:      pluginsdk.TypeString,
			Computed:  true,
			Sensitive: true,
		},

		"secondary_connection_string": {
			Type:      pluginsdk.TypeString,
			Computed:  true,
			Sensitive: true,
		},
	}
}

func (r NotificationHubAuthorizationRuleResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return hubs.ValidateNotificationHubAuthorizationRuleID
}

func (r NotificationHubAuthorizationRuleResource) ResourceType() string {
	return "azurerm_notification_hub_authorization_rule"
}

func (NotificationHubAuthorizationRuleResource) ModelObject() interface{} {
	return NotificationHubAuthorizationRuleResourceModel{}
}

func (r NotificationHubAuthorizationRuleResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient
			subscriptionId := metadata.Client.Account.SubscriptionId

			var config NotificationHubAuthorizationRuleResourceModel
			if err := metadata.Decode(&config); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			id := hubs.NewNotificationHubAuthorizationRuleID(subscriptionId, config.ResourceGroupName, config.NamespaceName, config.NotificationHubName, config.Name)

			existing, err := client.NotificationHubsGetAuthorizationRule(ctx, id)
			if err != nil {
				if !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
			}

			if !response.WasNotFound(existing.HttpResponse) {
				return tf.ImportAsExistsError("azurerm_notification_hub_authorization_rule", id.ID())
			}

			locks.ByName(id.NotificationHubName, notificationHubResourceName)
			defer locks.UnlockByName(id.NotificationHubName, notificationHubResourceName)

			locks.ByName(id.NamespaceName, notificationHubNamespaceResourceName)
			defer locks.UnlockByName(id.NamespaceName, notificationHubNamespaceResourceName)

			manage := config.Manage
			send := config.Send
			listen := config.Listen
			parameters := hubs.SharedAccessAuthorizationRuleResource{
				Properties: &hubs.SharedAccessAuthorizationRuleProperties{
					Rights: expandNotificationHubAuthorizationRuleRights(manage, send, listen),
				},
			}

			if _, err := client.NotificationHubsCreateOrUpdateAuthorizationRule(ctx, id, parameters); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			metadata.SetID(id)
			return nil
		},
	}
}

func (r NotificationHubAuthorizationRuleResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient

			id, err := hubs.ParseNotificationHubAuthorizationRuleID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			var config NotificationHubAuthorizationRuleResourceModel
			if err := metadata.Decode(&config); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			locks.ByName(id.NotificationHubName, notificationHubResourceName)
			defer locks.UnlockByName(id.NotificationHubName, notificationHubResourceName)

			locks.ByName(id.NamespaceName, notificationHubNamespaceResourceName)
			defer locks.UnlockByName(id.NamespaceName, notificationHubNamespaceResourceName)

			manage := config.Manage
			send := config.Send
			listen := config.Listen
			parameters := hubs.SharedAccessAuthorizationRuleResource{
				Properties: &hubs.SharedAccessAuthorizationRuleProperties{
					Rights: expandNotificationHubAuthorizationRuleRights(manage, send, listen),
				},
			}

			if _, err := client.NotificationHubsCreateOrUpdateAuthorizationRule(ctx, pointer.From(id), parameters); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			return nil
		},
	}
}

func (r NotificationHubAuthorizationRuleResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient

			id, err := hubs.ParseNotificationHubAuthorizationRuleID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			locks.ByName(id.NotificationHubName, notificationHubResourceName)
			defer locks.UnlockByName(id.NotificationHubName, notificationHubResourceName)

			locks.ByName(id.NamespaceName, notificationHubNamespaceResourceName)
			defer locks.UnlockByName(id.NamespaceName, notificationHubNamespaceResourceName)

			resp, err := client.NotificationHubsDeleteAuthorizationRule(ctx, pointer.From(id))
			if err != nil {
				if !response.WasNotFound(resp.HttpResponse) {
					return fmt.Errorf("deleting %s: %+v", pointer.From(id), err)
				}
			}

			return nil
		},
	}
}

func (r NotificationHubAuthorizationRuleResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(5 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient

			id, err := hubs.ParseNotificationHubAuthorizationRuleID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.NotificationHubsGetAuthorizationRule(ctx, pointer.From(id))
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					log.Printf("[DEBUG] %s was not found - removing from state", pointer.From(id))
					return metadata.MarkAsGone(id)
				}

				return fmt.Errorf("retrieving %s: %+v", pointer.From(id), err)
			}

			keysResp, err := client.NotificationHubsListKeys(ctx, pointer.From(id))
			if err != nil {
				return fmt.Errorf("listing access keys for %s: %+v", pointer.From(id), err)
			}

			config := NotificationHubAuthorizationRuleResourceModel{
				Name:                id.AuthorizationRuleName,
				NotificationHubName: id.NotificationHubName,
				NamespaceName:       id.NamespaceName,
				ResourceGroupName:   id.ResourceGroupName,
			}

			if model := resp.Model; model != nil {
				if props := model.Properties; props != nil {
					manage, send, listen := flattenNotificationHubAuthorizationRuleRights(&props.Rights)
					config.Manage = manage
					config.Send = send
					config.Listen = listen
				}
			}

			if keysModel := keysResp.Model; keysModel != nil {
				config.PrimaryAccssKey = pointer.From(keysModel.PrimaryKey)
				config.SecondaryAccessKey = pointer.From(keysModel.SecondaryKey)
				config.PrimaryConnectionString = pointer.From(keysModel.PrimaryConnectionString)
				config.SecondaryConnectionString = pointer.From(keysModel.SecondaryConnectionString)
			}

			return metadata.Encode(&config)
		},
	}
}

func (r NotificationHubAuthorizationRuleResource) CustomizeDiff() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: authorizationRuleCustomizeDiff,
	}
}
