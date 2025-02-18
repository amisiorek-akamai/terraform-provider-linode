package linode

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/linode/linodego"
	"github.com/linode/terraform-provider-linode/v2/linode/databaseaccesscontrols"
	"github.com/linode/terraform-provider-linode/v2/linode/databasemysql"
	"github.com/linode/terraform-provider-linode/v2/linode/databasemysqlbackups"
	"github.com/linode/terraform-provider-linode/v2/linode/databasepostgresql"
	"github.com/linode/terraform-provider-linode/v2/linode/domain"
	"github.com/linode/terraform-provider-linode/v2/linode/domainrecord"
	"github.com/linode/terraform-provider-linode/v2/linode/firewall"
	"github.com/linode/terraform-provider-linode/v2/linode/firewalldevice"
	"github.com/linode/terraform-provider-linode/v2/linode/helper"
	"github.com/linode/terraform-provider-linode/v2/linode/image"
	"github.com/linode/terraform-provider-linode/v2/linode/instance"
	"github.com/linode/terraform-provider-linode/v2/linode/instanceconfig"
	"github.com/linode/terraform-provider-linode/v2/linode/instancedisk"
	"github.com/linode/terraform-provider-linode/v2/linode/instanceip"
	"github.com/linode/terraform-provider-linode/v2/linode/instancesharedips"
	"github.com/linode/terraform-provider-linode/v2/linode/lke"
	"github.com/linode/terraform-provider-linode/v2/linode/nbconfig"
	"github.com/linode/terraform-provider-linode/v2/linode/nbnode"
	"github.com/linode/terraform-provider-linode/v2/linode/obj"
	"github.com/linode/terraform-provider-linode/v2/linode/objbucket"
	"github.com/linode/terraform-provider-linode/v2/linode/user"
	"github.com/linode/terraform-provider-linode/v2/linode/volume"
)

// Provider creates and manages the resources in a Linode configuration.
func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"token": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The token that allows you access to your Linode account",
			},
			"config_path": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"config_profile": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"url": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "The HTTP(S) API address of the Linode API to use.",
				ValidateFunc: validation.IsURLWithHTTPorHTTPS,
			},
			"ua_prefix": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "An HTTP User-Agent Prefix to prepend in API requests.",
			},
			"api_version": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The version of Linode API.",
			},

			"skip_instance_ready_poll": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Skip waiting for a linode_instance resource to be running.",
			},

			"skip_instance_delete_poll": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Skip waiting for a linode_instance resource to finish deleting.",
			},

			"skip_implicit_reboots": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "If true, Linode Instances will not be rebooted on config and interface changes.",
			},

			"disable_internal_cache": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Disable the internal caching system that backs certain Linode API requests.",
			},

			"min_retry_delay_ms": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Minimum delay in milliseconds before retrying a request.",
			},
			"max_retry_delay_ms": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Maximum delay in milliseconds before retrying a request.",
			},
			"event_poll_ms": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "The rate in milliseconds to poll for events.",
			},
			"lke_event_poll_ms": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "The rate in milliseconds to poll for LKE events.",
			},

			"lke_node_ready_poll_ms": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "The rate in milliseconds to poll for an LKE node to be ready.",
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"linode_database_mysql_backups": databasemysqlbackups.DataSource(),
			"linode_instances":              instance.DataSource(),
			"linode_lke_cluster":            lke.DataSource(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"linode_database_access_controls": databaseaccesscontrols.Resource(),
			"linode_database_mysql":           databasemysql.Resource(),
			"linode_database_postgresql":      databasepostgresql.Resource(),
			"linode_domain":                   domain.Resource(),
			"linode_domain_record":            domainrecord.Resource(),
			"linode_firewall":                 firewall.Resource(),
			"linode_firewall_device":          firewalldevice.Resource(),
			"linode_image":                    image.Resource(),
			"linode_instance":                 instance.Resource(),
			"linode_instance_config":          instanceconfig.Resource(),
			"linode_instance_disk":            instancedisk.Resource(),
			"linode_instance_ip":              instanceip.Resource(),
			"linode_instance_shared_ips":      instancesharedips.Resource(),
			"linode_lke_cluster":              lke.Resource(),
			"linode_nodebalancer_node":        nbnode.Resource(),
			"linode_nodebalancer_config":      nbconfig.Resource(),
			"linode_object_storage_bucket":    objbucket.Resource(),
			"linode_object_storage_object":    obj.Resource(),
			"linode_user":                     user.Resource(),
			"linode_volume":                   volume.Resource(),
		},
	}

	provider.ConfigureContextFunc = func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		terraformVersion := provider.TerraformVersion
		if terraformVersion == "" {
			// Terraform 0.12 introduced this field to the protocol
			// We can therefore assume that if it's missing it's 0.10 or 0.11
			terraformVersion = "0.11+compatible"
		}
		return providerConfigure(ctx, d, terraformVersion)
	}
	return provider
}

func handleDefault(config *helper.Config, d *schema.ResourceData) diag.Diagnostics {
	if v, ok := d.GetOk("token"); ok {
		config.AccessToken = v.(string)
	} else {
		config.AccessToken = os.Getenv("LINODE_TOKEN")
	}

	if v, ok := d.GetOk("api_version"); ok {
		config.APIVersion = v.(string)
	} else {
		config.APIVersion = os.Getenv("LINODE_API_VERSION")
	}

	if v, ok := d.GetOk("config_path"); ok {
		config.ConfigPath = v.(string)
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return diag.Errorf(
				"Failed to get user home directory: %s",
				err.Error(),
			)
		}
		config.ConfigPath = fmt.Sprintf("%s/.config/linode", homeDir)
	}

	if v, ok := d.GetOk("config_profile"); ok {
		config.ConfigProfile = v.(string)
	} else {
		config.ConfigProfile = "default"
	}

	if v, ok := d.GetOk("url"); ok {
		config.APIURL = v.(string)
	} else {
		config.APIURL = os.Getenv("LINODE_URL")
	}

	if v, ok := d.GetOk("ua_prefix"); ok {
		config.UAPrefix = v.(string)
	} else {
		config.UAPrefix = os.Getenv("LINODE_UA_PREFIX")
	}

	if v, ok := d.GetOk("event_poll_ms"); ok {
		config.EventPollMilliseconds = v.(int)
	} else {
		eventPollMs, err := strconv.Atoi(os.Getenv("LINODE_EVENT_POLL_MS"))
		if err != nil {
			eventPollMs = 4000
		}
		config.EventPollMilliseconds = eventPollMs
	}

	if v, ok := d.GetOk("lke_event_poll_ms"); ok {
		config.LKEEventPollMilliseconds = v.(int)
	} else {
		config.LKEEventPollMilliseconds = 3000
	}

	if v, ok := d.GetOk("lke_node_ready_poll_ms"); ok {
		config.LKENodeReadyPollMilliseconds = v.(int)
	} else {
		config.LKENodeReadyPollMilliseconds = 3000
	}

	return nil
}

func providerConfigure(
	ctx context.Context, d *schema.ResourceData, terraformVersion string,
) (interface{}, diag.Diagnostics) {
	config := &helper.Config{
		SkipInstanceReadyPoll:  d.Get("skip_instance_ready_poll").(bool),
		SkipInstanceDeletePoll: d.Get("skip_instance_delete_poll").(bool),
		SkipImplicitReboots:    d.Get("skip_implicit_reboots").(bool),

		DisableInternalCache: d.Get("disable_internal_cache").(bool),

		MinRetryDelayMilliseconds: d.Get("min_retry_delay_ms").(int),
		MaxRetryDelayMilliseconds: d.Get("max_retry_delay_ms").(int),
	}

	handleDefault(config, d)

	config.TerraformVersion = terraformVersion
	client, err := config.Client(ctx)
	if err != nil {
		return nil, diag.Errorf("failed to initialize client: %s", err)
	}

	// Ping the API for an empty response to verify the configuration works
	if _, err := client.ListTypes(ctx, linodego.NewListOptions(100, "")); err != nil {
		return nil, diag.Errorf("Error connecting to the Linode API: %s", err)
	}
	return &helper.ProviderMeta{
		Client: *client,
		Config: config,
	}, nil
}
