// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package notificationhub

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/tags"
	"github.com/hashicorp/go-azure-sdk/resource-manager/notificationhubs/2023-09-01/hubs"
	"github.com/hashicorp/go-azure-sdk/sdk/client/pollers"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/notificationhub/custompollers"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/notificationhub/migration"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
)

var notificationHubResourceName = "azurerm_notification_hub"

const (
	apnsProductionName     = "Production"
	apnsProductionEndpoint = "https://api.push.apple.com:443/3/device"
	apnsSandboxName        = "Sandbox"
	apnsSandboxEndpoint    = "https://api.development.push.apple.com:443/3/device"
)

var _ sdk.ResourceWithUpdate = NotificationHubResource{}

type NotificationHubResource struct{}

type NotificationHubResourceModel struct {
	Name              string                   `tfschema:"name"`
	NamespaceName     string                   `tfschema:"namespace_name"`
	ResourceGroupName string                   `tfschema:"resource_group_name"`
	Location          string                   `tfschema:"location"`
	ApnsCredential    []ApnsCredentialModel    `tfschema:"apns_credential"`
	BrowserCredential []BrowserCredentialModel `tfschema:"browser_credential"`
	GcmCredential     []GcmCredentialModel     `tfschema:"gcm_credential"`
	Tags              map[string]string        `tfschema:"tags"`
}

type ApnsCredentialModel struct {
	ApplicationMode string `tfschema:"application_mode"`
	BundleId        string `tfschema:"bundle_id"`
	KeyId           string `tfschema:"key_id"`
	TeamId          string `tfschema:"team_id"`
	Token           string `tfschema:"token"`
}

type BrowserCredentialModel struct {
	Subject         string `tfschema:"subject"`
	VapidPrivateKey string `tfschema:"vapid_private_key"`
	VapidPublicKey  string `tfschema:"vapid_public_key"`
}

type GcmCredentialModel struct {
	ApiKey string `tfschema:"api_key"`
}

func (r NotificationHubResource) StateUpgraders() sdk.StateUpgradeData {
	return sdk.StateUpgradeData{
		SchemaVersion: 1, // This field references the version which the state migration updates the schema to i.e. v0 -> v1
		Upgraders: map[int]pluginsdk.StateUpgrade{
			0: migration.NotificationHubResourceV0ToV1{},
		},
	}
}

func (r NotificationHubResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
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

		"location": commonschema.Location(),

		"apns_credential": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					// NOTE: APNS supports two modes, certificate auth (v1) and token auth (v2)
					// certificate authentication/v1 is marked for deprecation; as such we're not
					// supporting it at this time.
					"application_mode": {
						Type:     pluginsdk.TypeString,
						Required: true,
						ValidateFunc: validation.StringInSlice([]string{
							apnsProductionName,
							apnsSandboxName,
						}, false),
					},
					"bundle_id": {
						Type:     pluginsdk.TypeString,
						Required: true,
					},
					"key_id": {
						Type:     pluginsdk.TypeString,
						Required: true,
					},
					// Team ID (within Apple & the Portal) == "AppID" (within the API)
					"team_id": {
						Type:     pluginsdk.TypeString,
						Required: true,
					},
					"token": {
						Type:      pluginsdk.TypeString,
						Required:  true,
						Sensitive: true,
					},
				},
			},
		},

		"browser_credential": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			ForceNew: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"subject": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
					"vapid_private_key": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
						Sensitive:    true,
					},
					"vapid_public_key": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
		},

		"gcm_credential": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"api_key": {
						Type:      pluginsdk.TypeString,
						Required:  true,
						Sensitive: true,
					},
				},
			},
		},

		"tags": commonschema.Tags(),
	}
}

func (r NotificationHubResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{}
}

func (r NotificationHubResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return hubs.ValidateNotificationHubID
}

func (r NotificationHubResource) ResourceType() string {
	return "azurerm_notification_hub"
}

func (NotificationHubResource) ModelObject() interface{} {
	return NotificationHubResourceModel{}
}

func (r NotificationHubResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient
			subscriptionId := metadata.Client.Account.SubscriptionId

			var config NotificationHubResourceModel
			if err := metadata.Decode(&config); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			id := hubs.NewNotificationHubID(subscriptionId, config.ResourceGroupName, config.NamespaceName, config.Name)

			existing, err := client.NotificationHubsGet(ctx, id)
			if err != nil {
				if !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
			}

			if !response.WasNotFound(existing.HttpResponse) {
				return tf.ImportAsExistsError("azurerm_notification_hub", id.ID())
			}

			parameters := hubs.NotificationHubResource{
				Location: location.Normalize(config.Location),
				Properties: &hubs.NotificationHubProperties{
					ApnsCredential:    expandNotificationHubsAPNSCredentials(config.ApnsCredential),
					BrowserCredential: expandNotificationHubsBrowserCredentials(config.BrowserCredential),
					GcmCredential:     expandNotificationHubsGCMCredentials(config.GcmCredential),
				},
				Tags: pointer.To(config.Tags),
			}

			if _, err := client.NotificationHubsCreateOrUpdate(ctx, id, parameters); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			// Notification Hubs are eventually consistent
			log.Printf("[DEBUG] Waiting for %s to become available..", id)

			pollerType := custompollers.NewNotificationHubPoller(client, id)
			poller := pollers.NewPoller(pollerType, 10*time.Second, pollers.DefaultNumberOfDroppedConnectionsToAllow)
			if err := poller.PollUntilDone(ctx); err != nil {
				return err
			}

			metadata.SetID(id)
			return nil
		},
	}
}

func (r NotificationHubResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient

			var config NotificationHubResourceModel
			if err := metadata.Decode(&config); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			id, err := hubs.ParseNotificationHubID(metadata.ResourceData.Id())

			if err != nil {
				return err
			}

			existing, err := client.NotificationHubsGet(ctx, pointer.From(id))
			if err != nil {
				if response.WasNotFound(existing.HttpResponse) {
					return metadata.MarkAsGone(id)
				}

				return fmt.Errorf("retrieving %s: %+v", id, err)
			}

			parameters := hubs.NotificationHubPatchParameters{}

			if metadata.ResourceData.HasChange("tags") {
				parameters.Tags = pointer.To(config.Tags)
			}

			if metadata.ResourceData.HasChange("apns_credential") {
				parameters.Properties.ApnsCredential = expandNotificationHubsAPNSCredentials(config.ApnsCredential)
			}

			if metadata.ResourceData.HasChange("browser_credential") {
				parameters.Properties.BrowserCredential = expandNotificationHubsBrowserCredentials(config.BrowserCredential)
			}

			if metadata.ResourceData.HasChange("gcm_credential") {
				parameters.Properties.GcmCredential = expandNotificationHubsGCMCredentials(config.GcmCredential)
			}

			if _, err := client.NotificationHubsUpdate(ctx, pointer.From(id), parameters); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			// Notification Hubs are eventually consistent
			log.Printf("[DEBUG] Waiting for %s to become consistent..", id)

			pollerType := custompollers.NewNotificationHubPoller(client, pointer.From(id))
			poller := pollers.NewPoller(pollerType, 10*time.Second, pollers.DefaultNumberOfDroppedConnectionsToAllow)
			if err := poller.PollUntilDone(ctx); err != nil {
				return err
			}

			return nil
		},
	}
}

func (r NotificationHubResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(30 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient

			var config NotificationHubResourceModel
			if err := metadata.Decode(&config); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			id, err := hubs.ParseNotificationHubID(metadata.ResourceData.Id())

			if err != nil {
				return err
			}

			existing, err := client.NotificationHubsDelete(ctx, pointer.From(id))
			if err != nil {
				if !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("deleting %s: %+v", *id, err)
				}
			}

			return nil
		},
	}
}

func (r NotificationHubResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: *pluginsdk.DefaultTimeout(5 * time.Minute),

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient

			id, err := hubs.ParseNotificationHubID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.NotificationHubsGet(ctx, pointer.From(id))
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					log.Printf("[DEBUG] %s was not found - removing from state", *id)
					return metadata.MarkAsGone(id)
				}

				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			credentials, err := client.NotificationHubsGetPnsCredentials(ctx, *id)
			if err != nil {
				return fmt.Errorf("retrieving credentials for %s: %+v", *id, err)
			}

			output := NotificationHubResourceModel{
				Name:              id.NotificationHubName,
				NamespaceName:     id.NamespaceName,
				ResourceGroupName: id.ResourceGroupName,
			}

			if credentialsModel := credentials.Model; credentialsModel != nil {
				if props := credentialsModel.Properties; props != nil {
					output.ApnsCredential = []ApnsCredentialModel{flattenNotificationHubsAPNSCredentials(props.ApnsCredential)}
					output.BrowserCredential = []BrowserCredentialModel{flattenNotificationHubsBrowserCredentials(props.BrowserCredential)}
					output.GcmCredential = []GcmCredentialModel{flattenNotificationHubsGCMCredentials(props.GcmCredential)}
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

func (r NotificationHubResource) CustomizeDiff() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			// NOTE: the ForceNew is to workaround a bug in the Azure SDK where nil-values aren't sent to the API.
			// Bug: https://github.com/Azure/azure-sdk-for-go/issues/2246

			oAPNS, nAPNS := metadata.ResourceDiff.GetChange("apns_credential.#")
			oAPNSi := oAPNS.(int)
			nAPNSi := nAPNS.(int)
			if nAPNSi < oAPNSi {
				metadata.ResourceDiff.ForceNew("apns_credential")
			}

			oGCM, nGCM := metadata.ResourceDiff.GetChange("gcm_credential.#")
			oGCMi := oGCM.(int)
			nGCMi := nGCM.(int)
			if nGCMi < oGCMi {
				metadata.ResourceDiff.ForceNew("gcm_credential")
			}

			return nil
		},
	}
}

func resourceNotificationHubCreateUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).NotificationHubs.HubsClient
	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id := hubs.NewNotificationHubID(subscriptionId, d.Get("resource_group_name").(string), d.Get("namespace_name").(string), d.Get("name").(string))

	if d.IsNewResource() {
		if !meta.(*clients.Client).Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
			existing, err := client.NotificationHubsGet(ctx, id)
			if err != nil {
				if !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
			}

			if !response.WasNotFound(existing.HttpResponse) {
				return tf.ImportAsExistsError("azurerm_notification_hub", id.ID())
			}
		}
	}

	parameters := hubs.NotificationHubResource{
		Location: location.Normalize(d.Get("location").(string)),
		Properties: &hubs.NotificationHubProperties{
			ApnsCredential:    expandNotificationHubsAPNSCredentials(d.Get("apns_credential").([]interface{})),
			BrowserCredential: expandNotificationHubsBrowserCredentials(d.Get("browser_credential").([]interface{})),
			GcmCredential:     expandNotificationHubsGCMCredentials(d.Get("gcm_credential").([]interface{})),
		},
		Tags: tags.Expand(d.Get("tags").(map[string]interface{})),
	}

	if _, err := client.NotificationHubsCreateOrUpdate(ctx, id, parameters); err != nil {
		return fmt.Errorf("creating %s: %+v", id, err)
	}

	// Notification Hubs are eventually consistent
	log.Printf("[DEBUG] Waiting for %s to become available..", id)
	deadline, ok := ctx.Deadline()
	if !ok {
		return fmt.Errorf("internal-error: context had no deadline")
	}
	stateConf := &pluginsdk.StateChangeConf{
		Pending:                   []string{"404"},
		Target:                    []string{"200"},
		Refresh:                   notificationHubStateRefreshFunc(ctx, client, id),
		MinTimeout:                15 * time.Second,
		ContinuousTargetOccurence: 10,
		Timeout:                   time.Until(deadline),
	}
	if _, err := stateConf.WaitForStateContext(ctx); err != nil {
		return fmt.Errorf("waiting for %s to become available: %+v", id, err)
	}

	d.SetId(id.ID())
	return resourceNotificationHubRead(d, meta)
}

func notificationHubStateRefreshFunc(ctx context.Context, client *hubs.HubsClient, id hubs.NotificationHubId) pluginsdk.StateRefreshFunc {
	return func() (interface{}, string, error) {
		res, err := client.NotificationHubsGet(ctx, id)
		statusCode := "dropped connection"
		if res.HttpResponse != nil {
			statusCode = strconv.Itoa(res.HttpResponse.StatusCode)
		}

		if err != nil {
			if response.WasNotFound(res.HttpResponse) {
				return nil, statusCode, nil
			}

			return nil, "", fmt.Errorf("retrieving %s: %+v", id, err)
		}

		return res, statusCode, nil
	}
}

func resourceNotificationHubRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).NotificationHubs.HubsClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := hubs.ParseNotificationHubID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.NotificationHubsGet(ctx, *id)
	if err != nil {
		if response.WasNotFound(resp.HttpResponse) {
			log.Printf("[DEBUG] %s was not found - removing from state", *id)
			d.SetId("")
			return nil
		}

		return fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	credentials, err := client.NotificationHubsGetPnsCredentials(ctx, *id)
	if err != nil {
		return fmt.Errorf("retrieving credentials for %s: %+v", *id, err)
	}

	d.Set("name", id.NotificationHubName)
	d.Set("namespace_name", id.NamespaceName)
	d.Set("resource_group_name", id.ResourceGroupName)

	if credentialsModel := credentials.Model; credentialsModel != nil {
		if props := credentialsModel.Properties; props != nil {
			apns := flattenNotificationHubsAPNSCredentials(props.ApnsCredential)
			if setErr := d.Set("apns_credential", apns); setErr != nil {
				return fmt.Errorf("setting `apns_credential`: %+v", setErr)
			}
			browser := flattenNotificationHubsBrowserCredentials(props.BrowserCredential)
			if setErr := d.Set("browser_credential", browser); setErr != nil {
				return fmt.Errorf("setting `browser_credential`: %+v", setErr)
			}
			gcm := flattenNotificationHubsGCMCredentials(props.GcmCredential)
			if setErr := d.Set("gcm_credential", gcm); setErr != nil {
				return fmt.Errorf("setting `gcm_credential`: %+v", setErr)
			}
		}
	}

	if model := resp.Model; model != nil {
		d.Set("location", location.NormalizeNilable(&model.Location))

		return d.Set("tags", tags.Flatten(model.Tags))
	}

	return nil
}

func resourceNotificationHubDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).NotificationHubs.HubsClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := hubs.ParseNotificationHubID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.NotificationHubsDelete(ctx, *id)
	if err != nil {
		if !response.WasNotFound(resp.HttpResponse) {
			return fmt.Errorf("deleting %s: %+v", *id, err)
		}
	}

	return nil
}

func expandNotificationHubsAPNSCredentials(config []ApnsCredentialModel) *hubs.ApnsCredential {
	if len(config) == 0 {
		return nil
	}

	applicationMode := config[0].ApplicationMode
	bundleId := config[0].BundleId
	keyId := config[0].KeyId
	teamId := config[0].TeamId
	token := config[0].Token

	applicationEndpoints := map[string]string{
		apnsProductionName: apnsProductionEndpoint,
		apnsSandboxName:    apnsSandboxEndpoint,
	}
	endpoint := applicationEndpoints[applicationMode]

	credentials := hubs.ApnsCredential{
		Properties: hubs.ApnsCredentialProperties{
			AppId:    pointer.To(teamId),
			AppName:  pointer.To(bundleId),
			Endpoint: endpoint,
			KeyId:    pointer.To(keyId),
			Token:    pointer.To(token),
		},
	}
	return &credentials
}

func expandNotificationHubsBrowserCredentials(config []BrowserCredentialModel) *hubs.BrowserCredential {
	if len(config) == 0 {
		return nil
	}

	credentials := hubs.BrowserCredential{
		Properties: hubs.BrowserCredentialProperties{
			Subject:         config[0].Subject,
			VapidPrivateKey: config[0].VapidPrivateKey,
			VapidPublicKey:  config[0].VapidPublicKey,
		},
	}

	return &credentials
}

func flattenNotificationHubsAPNSCredentials(input *hubs.ApnsCredential) ApnsCredentialModel {
	output := ApnsCredentialModel{}

	if input == nil {
		return output
	}

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

	return output
}

func flattenNotificationHubsBrowserCredentials(input *hubs.BrowserCredential) BrowserCredentialModel {
	output := BrowserCredentialModel{}

	if input == nil {
		return output
	}

	output.Subject = input.Properties.Subject
	output.VapidPrivateKey = input.Properties.VapidPrivateKey
	output.VapidPublicKey = input.Properties.VapidPublicKey

	return output
}

func expandNotificationHubsGCMCredentials(inputs []GcmCredentialModel) *hubs.GcmCredential {
	if len(inputs) == 0 {
		return nil
	}

	apiKey := inputs[0].ApiKey
	credentials := hubs.GcmCredential{
		Properties: hubs.GcmCredentialProperties{
			GoogleApiKey: apiKey,
		},
	}
	return &credentials
}

func flattenNotificationHubsGCMCredentials(input *hubs.GcmCredential) GcmCredentialModel {
	output := GcmCredentialModel{}

	if input == nil {
		return output
	}

	output.ApiKey = input.Properties.GoogleApiKey

	return output
}
