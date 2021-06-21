package vault

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/GoogleCloudPlatform/terraformer/terraformutils"
	vault "github.com/hashicorp/vault/api"
)

type ServiceGenerator struct { //nolint
	terraformutils.Service
	client    *vault.Client
	mountType string
	resource  string
}

func (g *ServiceGenerator) setVaultClient() error {
	client, err := vault.NewClient(&vault.Config{Address: g.Args["address"].(string)})
	if err != nil {
		return err
	}
	if g.Args["token"] != "" {
		client.SetToken(g.Args["token"].(string))
	}
	g.client = client
	return nil
}

func (g *ServiceGenerator) InitResources() error {
	switch g.resource {
	case "secret_backend":
		return g.createSecretBackendResources()
	case "secret_backend_role":
		return g.createSecretBackendRoleResources()
	case "auth_backend":
		return g.createAuthBackendResources()
	case "auth_backend_role":
		return g.createAuthBackendEntityResources("role", "role")
	case "auth_backend_user":
		return g.createAuthBackendEntityResources("users", "user")
	case "auth_backend_group":
		return g.createAuthBackendEntityResources("groups", "group")
	case "policy":
		return g.createPolicyResources()
	default:
		return errors.New("unsupported service type. shouldn't ever reach here")
	}
}

func (g *ServiceGenerator) createSecretBackendResources() error {
	mounts, err := g.mountsByType()
	if err != nil {
		return err
	}
	for _, mount := range mounts {
		g.Resources = append(g.Resources,
			terraformutils.NewSimpleResource(
				mount,
				mount,
				fmt.Sprintf("vault_%s_secret_backend", g.mountType),
				g.ProviderName,
				[]string{}))
	}
	return nil
}

func (g *ServiceGenerator) createSecretBackendRoleResources() error {
	mounts, err := g.mountsByType()
	if err != nil {
		return err
	}
	for _, mount := range mounts {
		path := fmt.Sprintf("%s/roles", mount)
		s, err := g.client.Logical().List(path)
		if err != nil {
			log.Printf("error calling path %s: %s", path, err)
			continue
		}
		if s == nil {
			log.Printf("call to %s returned nil result", path)
			continue
		}
		roles, ok := s.Data["keys"]
		if !ok {
			log.Printf("no keys in call to %s", path)
			continue
		}
		for _, role := range roles.([]interface{}) {
			g.Resources = append(g.Resources,
				terraformutils.NewSimpleResource(
					fmt.Sprintf("%s/roles/%s", mount, role),
					fmt.Sprintf("%s_%s", mount, role),
					fmt.Sprintf("vault_%s_secret_backend_role", g.mountType),
					g.ProviderName,
					[]string{}))
		}
	}
	return nil
}

func (g *ServiceGenerator) mountsByType() ([]string, error) {
	mounts, err := g.client.Sys().ListMounts()
	if err != nil {
		return nil, err
	}
	var typeMounts []string
	for name, mount := range mounts {
		if mount.Type != g.mountType {
			continue
		}
		id := strings.ReplaceAll(name, "/", "")
		add := true
		for _, filter := range g.Filter {
			for _, value := range filter.AcceptableValues {
				add = value == id
				if add {
					break
				}
			}
			add = add && filter.FieldPath == "id" &&
				filter.IsApplicable(fmt.Sprintf("%s_secret_backend", mount.Type))
			if add {
				break
			}
		}
		if add {
			typeMounts = append(typeMounts, id)
		}
	}
	return typeMounts, nil
}

func (g *ServiceGenerator) createAuthBackendResources() error {
	backends, err := g.backendsByType()
	if err != nil {
		return err
	}
	for _, backend := range backends {
		g.Resources = append(g.Resources,
			terraformutils.NewSimpleResource(
				backend,
				backend,
				fmt.Sprintf("vault_%s_auth_backend", g.mountType),
				g.ProviderName,
				[]string{}))
	}
	return nil
}

func (g *ServiceGenerator) createAuthBackendEntityResources(apiEntity, tfEntity string) error {
	backends, err := g.backendsByType()
	if err != nil {
		return err
	}
	for _, backend := range backends {
		path := fmt.Sprintf("/auth/%s/%s", backend, apiEntity)
		s, err := g.client.Logical().List(path)
		if err != nil {
			log.Printf("error calling path %s: %s", path, err)
			continue
		}
		if s == nil {
			log.Printf("call to %s returned nil result", path)
			continue
		}
		names, ok := s.Data["keys"]
		if !ok {
			log.Printf("no keys in call to %s", path)
			continue
		}
		for _, name := range names.([]interface{}) {
			g.Resources = append(g.Resources,
				terraformutils.NewSimpleResource(
					fmt.Sprintf("auth/%s/%s/%s", backend, apiEntity, name),
					fmt.Sprintf("%s_%s", backend, name),
					fmt.Sprintf("vault_%s_auth_backend_%s", g.mountType, tfEntity),
					g.ProviderName,
					[]string{}))
		}
	}
	return nil
}

func (g *ServiceGenerator) backendsByType() ([]string, error) {
	authBackends, err := g.client.Sys().ListAuth()
	if err != nil {
		return nil, err
	}
	var typeBackends []string
	for name, authBackend := range authBackends {
		if authBackend.Type != g.mountType {
			continue
		}
		id := strings.ReplaceAll(name, "/", "")
		add := true
		for _, filter := range g.Filter {
			for _, value := range filter.AcceptableValues {
				add = value == id
				if add {
					break
				}
			}
			add = add && filter.FieldPath == "id" &&
				filter.IsApplicable(fmt.Sprintf("%s_auth_backend", authBackend.Type))
			if add {
				break
			}
		}
		if add {
			typeBackends = append(typeBackends, id)
		}
	}
	return typeBackends, nil
}

func (g *ServiceGenerator) createPolicyResources() error {
	policies, err := g.client.Sys().ListPolicies()
	if err != nil {
		return err
	}
	for _, policy := range policies {
		g.Resources = append(g.Resources,
			terraformutils.NewSimpleResource(
				policy,
				policy,
				"vault_policy",
				g.ProviderName,
				[]string{}))
	}
	return nil
}
