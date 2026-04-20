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
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-sdk/resource-manager/notificationhubs/2023-09-01/namespaces"
	"github.com/hashicorp/go-azure-sdk/sdk/client/pollers"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/notificationhub/custompollers"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/notificationhub/migration"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

var notificationHubNamespaceResourceName = "azurerm_notification_hub_namespace"

var _ sdk.ResourceWithUpdate = NotificationHubNamespaceResource{}

type NotificationHubNamespaceResource struct{}

type NotificationHubNamespaceResourceModel struct {
	Name                  string            `tfschema:"name"`
	ResourceGroupName     string            `tfschema:"resource_group_name"`
	Location              string            `tfschema:"location"`
	SkuName               string            `tfschema:"sku_name"`
	Enabled               bool              `tfschema:"enabled"`
	NamespaceType         string            `tfschema:"namespace_type"`
	ZoneRedundancyEnabled bool              `tfschema:"zone_redundancy_enabled"`
	ReplicationRegion     string            `tfschema:"replication_region"`
	ServicebusEndpoint    string            `tfschema:"servicebus_endpoint"`
	Tags                  map[string]string `tfschema:"tags"`
}

func (r NotificationHubNamespaceResource) StateUpgraders() sdk.StateUpgradeData {
	return sdk.StateUpgradeData{
		SchemaVersion: 1, // This field references the version which the state migration updates the schema to i.e. v0 -> v1
		Upgraders: map[int]pluginsdk.StateUpgrade{
			0: migration.NotificationHubNamespaceResourceV0ToV1{},
		},
	}
}

func resourceNotificationHubNamespaceCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).NotificationHubs.NamespacesClient
	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id := namespaces.NewNamespaceID(subscriptionId, d.Get("resource_group_name").(string), d.Get("name").(string))

	if !meta.(*clients.Client).Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
		existing, err := client.Get(ctx, id)
		if err != nil {
			if !response.WasNotFound(existing.HttpResponse) {
				return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
			}
		}

		if !response.WasNotFound(existing.HttpResponse) {
			return tf.ImportAsExistsError("azurerm_notification_hub_namespace", id.ID())
		}
	}

	zoneRedundancy := namespaces.ZoneRedundancyPreferenceDisabled
	if v, ok := d.GetOk("zone_redundancy_enabled"); ok && v.(bool) {
		zoneRedundancy = namespaces.ZoneRedundancyPreferenceEnabled
	}

	namespaceType := namespaces.NamespaceType(d.Get("namespace_type").(string))
	parameters := namespaces.NamespaceResource{
		Location: location.Normalize(d.Get("location").(string)),
		Sku: namespaces.Sku{
			Name: namespaces.SkuName(d.Get("sku_name").(string)),
func (r NotificationHubNamespaceResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"resource_group_name": commonschema.ResourceGroupName(),

		"location": commonschema.Location(),

		"sku_name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ValidateFunc: validation.StringInSlice([]string{
				string(namespaces.SkuNameBasic),
				string(namespaces.SkuNameFree),
				string(namespaces.SkuNameStandard),
			}, false),
		},

		"enabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			ForceNew: true,
			Default:  true,
		},

		"namespace_type": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
			ValidateFunc: validation.StringInSlice([]string{
				string(namespaces.NamespaceTypeMessaging),
				string(namespaces.NamespaceTypeNotificationHub),
			}, false),
		},

		"zone_redundancy_enabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
			ForceNew: true,
		},

		"replication_region": {
			Type:             pluginsdk.TypeString,
			Optional:         true,
			ForceNew:         true,
			Default:          namespaces.ReplicationRegionDefault,
			ValidateFunc:     validation.StringInSlice(namespaces.PossibleValuesForReplicationRegion(), true),
			DiffSuppressFunc: location.DiffSuppressFunc,
		},

		"tags": commonschema.Tags(),
	}
}

func (r NotificationHubNamespaceResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"servicebus_endpoint": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},
	}
}

func (r NotificationHubNamespaceResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return namespaces.ValidateNamespaceID
}

func (r NotificationHubNamespaceResource) ResourceType() string {
	return notificationHubNamespaceResourceName
}

func (NotificationHubNamespaceResource) ModelObject() interface{} {
	return &NotificationHubNamespaceResourceModel{}
}

func (r NotificationHubNamespaceResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.NamespacesClient
			subscriptionId := metadata.Client.Account.SubscriptionId

			var config NotificationHubNamespaceResourceModel
			if err := metadata.Decode(&config); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			id := namespaces.NewNamespaceID(subscriptionId, config.ResourceGroupName, config.Name)

			existing, err := client.Get(ctx, id)
			if err != nil {
				if !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
			}

			if !response.WasNotFound(existing.HttpResponse) {
				return tf.ImportAsExistsError("azurerm_notification_hub_namespace", id.ID())
			}

			zoneRedundancy := namespaces.ZoneRedundancyPreferenceDisabled
			if config.ZoneRedundancyEnabled {
				zoneRedundancy = namespaces.ZoneRedundancyPreferenceEnabled
			}

			namespaceType := namespaces.NamespaceType(config.NamespaceType)
			parameters := namespaces.NamespaceResource{
				Location: location.Normalize(config.Location),
				Sku: namespaces.Sku{
					Name: namespaces.SkuName(config.SkuName),
				},
				Properties: &namespaces.NamespaceProperties{
					NamespaceType:  pointer.To(namespaceType),
					Enabled:        pointer.To(config.Enabled),
					ZoneRedundancy: pointer.To(zoneRedundancy),
				},
				Tags: pointer.To(config.Tags),
			}

			parameters.Properties.ReplicationRegion = pointer.To(namespaces.ReplicationRegion(location.Normalize(config.ReplicationRegion)))

			if _, err := client.CreateOrUpdate(ctx, id, parameters); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			log.Printf("[DEBUG] Waiting for %s to be created..", id)

			pollerType := custompollers.NewNotificationHubNamespacePoller(client, id)
			poller := pollers.NewPoller(pollerType, 10*time.Second, pollers.DefaultNumberOfDroppedConnectionsToAllow)
			if err := poller.PollUntilDone(ctx); err != nil {
				return err
			}

			metadata.SetID(id)
			return nil
		},
	}
}

func (r NotificationHubNamespaceResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.NamespacesClient

			id, err := namespaces.ParseNamespaceID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			var config NotificationHubNamespaceResourceModel
			if err := metadata.Decode(&config); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			parameters := namespaces.NamespacePatchParameters{
				Properties: &namespaces.NamespaceProperties{
					NamespaceType: pointer.To(namespaces.NamespaceType(config.NamespaceType)),
					Enabled:       pointer.To(config.Enabled),
				},
			}

			if metadata.ResourceData.HasChange("sku_name") {
				parameters.Sku = &namespaces.Sku{
					Name: namespaces.SkuName(config.SkuName),
				}
			}

			if metadata.ResourceData.HasChange("tags") {
				parameters.Tags = pointer.To(config.Tags)
			}

			if _, err := client.Update(ctx, *id, parameters); err != nil {
				return fmt.Errorf("updating %s: %+v", id, err)
			}

			return nil
		},
	}
}

func (r NotificationHubNamespaceResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.NamespacesClient

			id, err := namespaces.ParseNamespaceID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.Delete(ctx, *id)
			if err != nil {
				if !response.WasNotFound(resp.HttpResponse) {
					return fmt.Errorf("deleting %s: %+v", *id, err)
				}
			}

			return nil
		},
	}
}

func (r NotificationHubNamespaceResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(5 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.NamespacesClient

			id, err := namespaces.ParseNamespaceID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.Get(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					log.Printf("[DEBUG] %s was not found - removing from state!", *id)
					return metadata.MarkAsGone(id)
				}

				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			config := NotificationHubNamespaceResourceModel{
				Name:              id.NamespaceName,
				ResourceGroupName: id.ResourceGroupName,
			}

			if model := resp.Model; model != nil {
				config.Location = location.NormalizeNilable(&model.Location)
				config.SkuName = string(model.Sku.Name)
				if props := model.Properties; props != nil {
					config.NamespaceType = string(pointer.From(props.NamespaceType))
					config.Enabled = pointer.From(props.Enabled)
					config.ServicebusEndpoint = pointer.From(props.ServiceBusEndpoint)
					config.ZoneRedundancyEnabled = pointer.From(props.ZoneRedundancy) == namespaces.ZoneRedundancyPreferenceEnabled
					replicationRegion := string(namespaces.ReplicationRegionDefault)
					if v := pointer.FromEnum(props.ReplicationRegion); v != "" {
						replicationRegion = v
					}
					config.ReplicationRegion = location.Normalize(replicationRegion)
				}

				config.Tags = pointer.From(model.Tags)
			}
			return metadata.Encode(&config)
		},
	}
}
