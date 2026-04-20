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
	"github.com/hashicorp/go-azure-sdk/resource-manager/notificationhubs/2023-09-01/namespaces"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

var _ sdk.DataSource = NotificationHubNamespaceDataSource{}

type NotificationHubNamespaceDataSource struct{}

type NotificationHubNamespaceDataSourceModel struct {
	Name                  string                                       `tfschema:"name"`
	ResourceGroupName     string                                       `tfschema:"resource_group_name"`
	Location              string                                       `tfschema:"location"`
	Sku                   []NotificationHubNamespaceDataSourceSkuModel `tfschema:"sku"`
	Enabled               bool                                         `tfschema:"enabled"`
	NamespaceType         string                                       `tfschema:"namespace_type"`
	ZoneRedundancyEnabled bool                                         `tfschema:"zone_redundancy_enabled"`
	ReplicationRegion     string                                       `tfschema:"replication_region"`
	ServicebusEndpoint    string                                       `tfschema:"servicebus_endpoint"`
	Tags                  map[string]string                            `tfschema:"tags"`
}

type NotificationHubNamespaceDataSourceSkuModel struct {
	Name string `tfschema:"name"`
}

func (r NotificationHubNamespaceDataSource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
		},

		"resource_group_name": commonschema.ResourceGroupNameForDataSource(),
	}
}

func (r NotificationHubNamespaceDataSource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"location": commonschema.LocationComputed(),

		"sku": {
			Type:     pluginsdk.TypeList,
			Computed: true,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"name": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
				},
			},
		},

		"enabled": {
			Type:     pluginsdk.TypeBool,
			Computed: true,
		},

		"namespace_type": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},

		"tags": commonschema.TagsDataSource(),

		"servicebus_endpoint": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},

		"zone_redundancy_enabled": {
			Type:     pluginsdk.TypeBool,
			Computed: true,
		},

		"replication_region": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},
	}
}

func (r NotificationHubNamespaceDataSource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return namespaces.ValidateNamespaceID
}

func (r NotificationHubNamespaceDataSource) ResourceType() string {
	return "azurerm_notification_hub_namespace"
}

func (NotificationHubNamespaceDataSource) ModelObject() interface{} {
	return &NotificationHubNamespaceDataSourceModel{}
}

func (r NotificationHubNamespaceDataSource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(5 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.NamespacesClient
			subscriptionId := metadata.Client.Account.SubscriptionId

			var config NotificationHubNamespaceDataSourceModel
			if err := metadata.Decode(&config); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			id := namespaces.NewNamespaceID(subscriptionId, config.ResourceGroupName, config.Name)
			resp, err := client.Get(ctx, id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return fmt.Errorf("%s was not found", id)
				}

				return fmt.Errorf("retrieving %s: %+v", id, err)
			}

			metadata.SetID(id)

			output := NotificationHubNamespaceDataSourceModel{
				Name:              id.NamespaceName,
				ResourceGroupName: id.ResourceGroupName,
			}

			if model := resp.Model; model != nil {
				output.Location = location.NormalizeNilable(&model.Location)
				output.Sku = []NotificationHubNamespaceDataSourceSkuModel{{
					Name: string(model.Sku.Name),
				}}

				if props := model.Properties; props != nil {
					output.Enabled = pointer.From(props.Enabled)
					output.NamespaceType = string(pointer.From(props.NamespaceType))
					output.ServicebusEndpoint = pointer.From(props.ServiceBusEndpoint)
					output.ZoneRedundancyEnabled = pointer.From(props.ZoneRedundancy) == namespaces.ZoneRedundancyPreferenceEnabled
					replicationRegion := string(namespaces.ReplicationRegionDefault)
					if v := pointer.FromEnum(props.ReplicationRegion); v != "" {
						replicationRegion = v
					}
					output.ReplicationRegion = location.Normalize(replicationRegion)
				}

				output.Tags = pointer.From(model.Tags)
				return metadata.Encode(&output)
			}

			return nil
		},
	}
}
